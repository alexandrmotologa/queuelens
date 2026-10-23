package server

import (
	"encoding/json"
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
	redisAddr string,
	isDemo bool,
) *Server {
	return &Server{
		rdb:         rdb,
		registry:    registry,
		broadcaster: broadcaster,
		metrics:     metrics,
		stallAudit:  stallAudit,
		replayer:    dlq.NewReplayer(registry, rdb),
		redisAddr:   redisAddr,
		isDemo:      isDemo,
	}
}

// RegisterRoutes registers all API endpoints on the given ServeMux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/queues", s.handleQueues)
	mux.HandleFunc("/api/queues/", s.handleQueueRoute)
	mux.HandleFunc("/api/jobs/", s.handleJobRoute)
	mux.HandleFunc("/api/dlq/clusters", s.handleDLQClusters)
	mux.HandleFunc("/api/dlq/replay", s.handleDLQReplay)
	mux.HandleFunc("/api/stalls", s.handleStalls)
	mux.HandleFunc("/api/stalls/unlock", s.handleStallsUnlock)
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
	ctx := r.Context()
	info, err := client.GetServerInfo(ctx, s.rdb)
	if err != nil {
		info = &client.ServerInfo{
			Mode:    "unknown",
			Version: "unknown",
		}
	}

	response := map[string]interface{}{
		"connected": true,
		"redisAddr": s.redisAddr,
		"isDemo":    s.isDemo,
		"server":    info,
		"timestamp": time.Now().Unix(),
	}
	writeJSON(w, http.StatusOK, response)
}

// GET /api/queues
func (s *Server) handleQueues(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queues, err := s.registry.DiscoverAll(ctx, s.rdb)
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

	ctx := r.Context()

	// GET /api/queues/{type}/{name}/jobs?state=waiting&offset=0&limit=50
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

		jobs, total, err := adapter.ListJobs(ctx, s.rdb, qName, state, offset, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"jobs":   jobs,
			"total":  total,
			"offset": offset,
			"limit":  limit,
			"state":  state,
		})
		return
	}

	// POST /api/queues/{type}/{name}/purge?state=failed
	if len(parts) >= 3 && parts[2] == "purge" && r.Method == http.MethodPost {
		state := adapters.JobState(r.URL.Query().Get("state"))
		count, err := adapter.PurgeQueue(ctx, s.rdb, qName, state)
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

	ctx := r.Context()

	// GET /api/jobs/{type}/{name}/{id}
	if len(parts) == 3 && r.Method == http.MethodGet {
		job, err := adapter.GetJob(ctx, s.rdb, qName, jobID)
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

		err := adapter.RetryJob(ctx, s.rdb, qName, jobID, payloadBytes)
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
		err := adapter.DeleteJob(ctx, s.rdb, qName, jobID)
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
	ctx := r.Context()
	queues, err := s.registry.DiscoverAll(ctx, s.rdb)
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

		jobs, _, err := adapter.ListJobs(ctx, s.rdb, q.Name, adapters.StateFailed, 0, 500)
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

// GET /api/metrics
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	summary := s.metrics.GetSummary()

	// Fill aggregate counts across discovered queues
	ctx := r.Context()
	queues, err := s.registry.DiscoverAll(ctx, s.rdb)
	if err == nil {
		summary.TotalActiveQueues = len(queues)
		for _, q := range queues {
			summary.TotalWaitingJobs += q.Stats.Waiting
			summary.TotalActiveJobs += q.Stats.Active
			summary.TotalFailedJobs += q.Stats.Failed
		}
	}

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
