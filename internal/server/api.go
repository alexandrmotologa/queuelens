package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/queuelens/internal/adapters"
	"github.com/alexandrmotologa/queuelens/internal/client"
	"github.com/alexandrmotologa/queuelens/internal/dlq"
	"github.com/alexandrmotologa/queuelens/internal/engine"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Server provides HTTP REST APIs and WebSocket real-time feeds.
type Server struct {
	rdb         redis.UniversalClient
	registry    *adapters.Registry
	broadcaster *engine.EventBroadcaster
	metrics     *engine.MetricsEngine
	stallAudit  *engine.StallAuditor
	replayer    *dlq.Replayer
	notifier    *engine.WebhookNotifier
	redisAddr   string
	isDemo      bool
	mu          sync.RWMutex
}

// NewServer configures a new Server instance.
func NewServer(
	rdb redis.UniversalClient,
	registry *adapters.Registry,
	broadcaster *engine.EventBroadcaster,
	metrics *engine.MetricsEngine,
	stallAudit *engine.StallAuditor,
	notifier *engine.WebhookNotifier,
	redisAddr string,
	isDemo bool,
) *Server {
	if notifier == nil {
		notifier = engine.NewWebhookNotifier("")
	}
	return &Server{
		rdb:         rdb,
		registry:    registry,
		broadcaster: broadcaster,
		metrics:     metrics,
		stallAudit:  stallAudit,
		replayer:    dlq.NewReplayer(registry, rdb),
		notifier:    notifier,
		redisAddr:   redisAddr,
		isDemo:      isDemo,
	}
}

// RegisterRoutes registers all API endpoints on the given ServeMux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/connect", s.handleConnect)
	mux.HandleFunc("/api/queues", s.handleQueues)
	mux.HandleFunc("/api/queues/", s.handleQueueRoute)
	mux.HandleFunc("/api/jobs/", s.handleJobRoute)
	mux.HandleFunc("/api/dlq/clusters", s.handleDLQClusters)
	mux.HandleFunc("/api/dlq/replay", s.handleDLQReplay)
	mux.HandleFunc("/api/dlq/export", s.handleDLQExport)
	mux.HandleFunc("/api/dlq/import", s.handleDLQImport)
	mux.HandleFunc("/api/dlq/batch-transform", s.handleDLQBatchTransform)
	mux.HandleFunc("/api/stalls", s.handleStalls)
	mux.HandleFunc("/api/stalls/unlock", s.handleStallsUnlock)
	mux.HandleFunc("/api/alerts/config", s.handleAlertsConfig)
	mux.HandleFunc("/api/alerts/test", s.handleAlertsTest)
	mux.HandleFunc("/api/metrics", s.handleMetrics)
	mux.HandleFunc("/api/ws", s.handleWebSocket)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// GET /api/status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	clientInst := s.rdb
	addr := s.redisAddr
	demo := s.isDemo
	s.mu.RUnlock()

	ctx := r.Context()
	info, err := client.GetServerInfo(ctx, clientInst)
	if err != nil {
		info = &client.ServerInfo{
			Mode:    "unknown",
			Version: "unknown",
		}
	}

	response := map[string]interface{}{
		"connected": true,
		"redisAddr": addr,
		"isDemo":    demo,
		"server":    info,
		"timestamp": time.Now().Unix(),
	}
	writeJSON(w, http.StatusOK, response)
}

// POST /api/connect
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request JSON")
		return
	}

	targetURL := strings.TrimSpace(req.URL)
	if targetURL == "" {
		writeError(w, http.StatusBadRequest, "URL cannot be empty")
		return
	}

	newClient, err := client.NewClient(client.Config{
		URL:     targetURL,
		Timeout: 4 * time.Second,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid client configuration: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx, newClient); err != nil {
		newClient.Close()
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to connect to Redis at %s: %v", targetURL, err))
		return
	}

	// Hot-swap the active Redis connection
	s.mu.Lock()
	oldClient := s.rdb
	s.rdb = newClient
	s.redisAddr = targetURL
	s.isDemo = false
	s.replayer = dlq.NewReplayer(s.registry, newClient)
	s.stallAudit.SetRedisClient(newClient)
	s.mu.Unlock()

	if oldClient != nil {
		_ = oldClient.Close()
	}

	info, _ := client.GetServerInfo(ctx, newClient)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"redisAddr": targetURL,
		"server":    info,
	})
}

// GET /api/queues
func (s *Server) handleQueues(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	clientInst := s.rdb
	s.mu.RUnlock()

	ctx := r.Context()
	queues, err := s.registry.DiscoverAll(ctx, clientInst)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"queues": queues,
		"total":  len(queues),
	})
}

// /api/queues/{type}/{name}/...
func (s *Server) handleQueueRoute(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/queues/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		writeError(w, http.StatusBadRequest, "invalid queue path, expected /api/queues/{type}/{name}")
		return
	}

	qType := adapters.QueueType(parts[0])
	qName := parts[1]

	adapter, err := s.registry.Get(qType)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.RLock()
	clientInst := s.rdb
	s.mu.RUnlock()

	ctx := r.Context()

	// GET /api/queues/{type}/{name}/jobs?state=waiting&offset=0&limit=50&search=...&since=...
	if len(parts) >= 3 && parts[2] == "jobs" && r.Method == http.MethodGet {
		stateStr := r.URL.Query().Get("state")
		if stateStr == "" {
			stateStr = "waiting"
		}
		state := adapters.JobState(stateStr)

		offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
		limit, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64)
		if limit <= 0 || limit > 100 {
			limit = 50
		}

		search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
		sinceStr := r.URL.Query().Get("since")
		var sinceTime time.Time
		if sinceStr != "" {
			if mins, err := strconv.Atoi(sinceStr); err == nil && mins > 0 {
				sinceTime = time.Now().Add(-time.Duration(mins) * time.Minute)
			}
		}

		jobs, total, err := adapter.ListJobs(ctx, clientInst, qName, state, offset, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Apply deep payload & time filters
		var filtered []adapters.Job
		for _, job := range jobs {
			if !sinceTime.IsZero() && !job.EnqueuedAt.IsZero() && job.EnqueuedAt.Before(sinceTime) {
				continue
			}
			if search != "" {
				rawJSON := job.RawPayload
				if rawJSON == "" {
					b, _ := json.Marshal(job.Payload)
					rawJSON = string(b)
				}
				matchID := strings.Contains(strings.ToLower(job.ID), search)
				matchName := strings.Contains(strings.ToLower(job.Name), search)
				matchError := strings.Contains(strings.ToLower(job.ErrorReason), search)
				matchPayload := strings.Contains(strings.ToLower(rawJSON), search)

				if !matchID && !matchName && !matchError && !matchPayload {
					continue
				}
			}
			filtered = append(filtered, job)
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"jobs":   filtered,
			"total":  total,
			"offset": offset,
			"limit":  limit,
			"state":  state,
		})
		return
	}

	// POST /api/queues/{type}/{name}/pause
	if len(parts) >= 3 && parts[2] == "pause" && r.Method == http.MethodPost {
		err := adapter.PauseQueue(ctx, clientInst, qName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"queue":   qName,
			"paused":  true,
		})
		return
	}

	// POST /api/queues/{type}/{name}/resume
	if len(parts) >= 3 && parts[2] == "resume" && r.Method == http.MethodPost {
		err := adapter.ResumeQueue(ctx, clientInst, qName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"queue":   qName,
			"paused":  false,
		})
		return
	}

	// POST /api/queues/{type}/{name}/drain
	if len(parts) >= 3 && parts[2] == "drain" && r.Method == http.MethodPost {
		count, err := adapter.PurgeQueue(ctx, clientInst, qName, adapters.StateWaiting)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"queue":   qName,
			"drained": count,
		})
		return
	}

	// POST /api/queues/{type}/{name}/purge?state=failed
	if len(parts) >= 3 && parts[2] == "purge" && r.Method == http.MethodPost {
		state := adapters.JobState(r.URL.Query().Get("state"))
		count, err := adapter.PurgeQueue(ctx, clientInst, qName, state)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"purged": count,
			"queue":  qName,
			"state":  state,
		})
		return
	}

	writeError(w, http.StatusNotFound, "endpoint not found")
}

// /api/jobs/{type}/{name}/{id}/...
func (s *Server) handleJobRoute(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		writeError(w, http.StatusBadRequest, "invalid job path, expected /api/jobs/{type}/{name}/{id}")
		return
	}

	qType := adapters.QueueType(parts[0])
	qName := parts[1]
	jobID := parts[2]

	adapter, err := s.registry.Get(qType)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.RLock()
	clientInst := s.rdb
	s.mu.RUnlock()

	ctx := r.Context()

	// GET /api/jobs/{type}/{name}/{id}
	if len(parts) == 3 && r.Method == http.MethodGet {
		job, err := adapter.GetJob(ctx, clientInst, qName, jobID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, job)
		return
	}

	// POST /api/jobs/{type}/{name}/{id}/retry
	if len(parts) == 4 && parts[3] == "retry" && r.Method == http.MethodPost {
		body, _ := io.ReadAll(r.Body)
		var reqPayload struct {
			Payload string `json:"payload"`
		}
		var payloadBytes []byte
		if len(body) > 0 {
			if err := json.Unmarshal(body, &reqPayload); err == nil && reqPayload.Payload != "" {
				payloadBytes = []byte(reqPayload.Payload)
			} else {
				payloadBytes = body
			}
		}

		err := adapter.RetryJob(ctx, clientInst, qName, jobID, payloadBytes)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"jobId":   jobID,
			"message": "Job successfully moved to waiting state",
		})
		return
	}

	// DELETE /api/jobs/{type}/{name}/{id}
	if len(parts) == 3 && r.Method == http.MethodDelete {
		err := adapter.DeleteJob(ctx, clientInst, qName, jobID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"jobId":   jobID,
		})
		return
	}

	writeError(w, http.StatusNotFound, "endpoint not found")
}

// GET /api/dlq/clusters
func (s *Server) handleDLQClusters(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	clientInst := s.rdb
	s.mu.RUnlock()

	ctx := r.Context()
	queues, err := s.registry.DiscoverAll(ctx, clientInst)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var allFailedJobs []adapters.Job
	for _, q := range queues {
		if q.Stats.Failed == 0 {
			continue
		}
		adapter, err := s.registry.Get(q.Type)
		if err != nil {
			continue
		}

		jobs, _, err := adapter.ListJobs(ctx, clientInst, q.Name, adapters.StateFailed, 0, 500)
		if err == nil {
			allFailedJobs = append(allFailedJobs, jobs...)
		}
	}

	clusters := dlq.ClusterJobs(allFailedJobs)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"clusters":        clusters,
		"totalClusters":   len(clusters),
		"totalFailedJobs": len(allFailedJobs),
	})
}

// POST /api/dlq/replay
func (s *Server) handleDLQReplay(w http.ResponseWriter, r *http.Request) {
	var req dlq.ReplayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request: "+err.Error())
		return
	}

	if len(req.JobIDs) == 0 {
		writeError(w, http.StatusBadRequest, "jobIds array must not be empty")
		return
	}

	ctx := r.Context()
	result, err := s.replayer.ReplayBatch(ctx, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GET /api/dlq/export?format=json|csv&clusterId=...
func (s *Server) handleDLQExport(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	clientInst := s.rdb
	s.mu.RUnlock()

	ctx := r.Context()
	queues, err := s.registry.DiscoverAll(ctx, clientInst)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var allFailed []adapters.Job
	for _, q := range queues {
		if q.Stats.Failed > 0 {
			adapter, _ := s.registry.Get(q.Type)
			jobs, _, _ := adapter.ListJobs(ctx, clientInst, q.Name, adapters.StateFailed, 0, 1000)
			allFailed = append(allFailed, jobs...)
		}
	}

	clusterID := r.URL.Query().Get("clusterId")
	if clusterID != "" {
		clusters := dlq.ClusterJobs(allFailed)
		var targetIDs []string
		for _, c := range clusters {
			if c.ID == clusterID {
				targetIDs = c.JobIDs
				break
			}
		}
		idMap := make(map[string]bool)
		for _, id := range targetIDs {
			idMap[id] = true
		}
		var filtered []adapters.Job
		for _, j := range allFailed {
			if idMap[j.ID] {
				filtered = append(filtered, j)
			}
		}
		allFailed = filtered
	}

	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "csv" {
		csvBytes, err := dlq.ExportCSV(allFailed)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="queuelens-dlq-export.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(csvBytes)
		return
	}

	jsonBytes, err := dlq.ExportJSON(allFailed)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="queuelens-dlq-export.json"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jsonBytes)
}

// POST /api/dlq/import
func (s *Server) handleDLQImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil || len(data) == 0 {
		writeError(w, http.StatusBadRequest, "empty bundle data")
		return
	}

	s.mu.RLock()
	clientInst := s.rdb
	s.mu.RUnlock()

	ctx := r.Context()
	result, err := dlq.ImportAndReplay(ctx, s.registry, clientInst, data)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// POST /api/dlq/batch-transform
func (s *Server) handleDLQBatchTransform(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req dlq.BatchTransformRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}

	s.mu.RLock()
	clientInst := s.rdb
	s.mu.RUnlock()

	ctx := r.Context()
	res, err := dlq.BatchTransform(ctx, s.registry, clientInst, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// GET /api/stalls
func (s *Server) handleStalls(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	warnings, err := s.stallAudit.AuditNow(ctx)
	if err != nil {
		warnings = s.stallAudit.GetWarnings()
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"stalls": warnings,
		"count":  len(warnings),
	})
}

// POST /api/stalls/unlock
func (s *Server) handleStallsUnlock(w http.ResponseWriter, r *http.Request) {
	var body struct {
		QueueType adapters.QueueType `json:"queueType"`
		Queue     string             `json:"queue"`
		LockKey   string             `json:"lockKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	err := s.stallAudit.ForceUnlock(r.Context(), body.QueueType, body.Queue, body.LockKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"lockKey": body.LockKey,
		"message": "Lock key released successfully",
	})
}

// GET & POST /api/alerts/config
func (s *Server) handleAlertsConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"webhookUrl": s.notifier.GetWebhookURL(),
			"configured": s.notifier.GetWebhookURL() != "",
		})
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			WebhookURL string `json:"webhookUrl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		s.notifier.SetWebhookURL(strings.TrimSpace(req.WebhookURL))
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success":    true,
			"webhookUrl": s.notifier.GetWebhookURL(),
		})
		return
	}

	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

// POST /api/alerts/test
func (s *Server) handleAlertsTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	err := s.notifier.SendAlert(r.Context(), engine.AlertEvent{
		Type:      "test_alert",
		Title:     "QueueLens Test Alert",
		Message:   "Test webhook notification dispatched successfully from QueueLens studio",
		Severity:  engine.SeverityInfo,
		Timestamp: time.Now(),
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to dispatch test webhook: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Test webhook alert sent successfully",
	})
}

// GET /api/metrics
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	summary := s.metrics.GetSummary()

	// Fill aggregate counts across discovered queues
	s.mu.RLock()
	clientInst := s.rdb
	s.mu.RUnlock()

	ctx := r.Context()
	queues, err := s.registry.DiscoverAll(ctx, clientInst)
	var totalWaiting int64
	if err == nil {
		summary.TotalActiveQueues = len(queues)
		for _, q := range queues {
			totalWaiting += q.Stats.Waiting
			summary.TotalActiveJobs += q.Stats.Active
			summary.TotalFailedJobs += q.Stats.Failed
		}
	}

	// Compute Time-to-Empty and SLA breaches
	s.metrics.ComputeSLA(&summary, totalWaiting)

	writeJSON(w, http.StatusOK, summary)
}

// GET /api/ws
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ch := s.broadcaster.Subscribe()
	defer s.broadcaster.Unsubscribe(ch)

	// Send initial recent event history
	recent := s.broadcaster.GetRecentHistory()
	_ = conn.WriteJSON(map[string]interface{}{
		"type":    "history",
		"payload": recent,
	})

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteJSON(map[string]interface{}{
				"type":    "event",
				"payload": event,
			}); err != nil {
				return
			}
		case <-ticker.C:
			summary := s.metrics.GetSummary()
			if err := conn.WriteJSON(map[string]interface{}{
				"type":    "metrics",
				"payload": summary,
			}); err != nil {
				return
			}
		}
	}
}
