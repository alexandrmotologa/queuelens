package dlq

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alexandrmotologa/queuelens/internal/adapters"
	"github.com/redis/go-redis/v9"
)

// DLQBundle is the portable snapshot structure for exporting and importing failed jobs.
type DLQBundle struct {
	Version    string         `json:"version"`
	ExportedAt time.Time      `json:"exportedAt"`
	TotalJobs  int            `json:"totalJobs"`
	Jobs       []adapters.Job `json:"jobs"`
}

// ExportJSON generates a formatted JSON document of failed jobs.
func ExportJSON(jobs []adapters.Job) ([]byte, error) {
	bundle := DLQBundle{
		Version:    "1.0",
		ExportedAt: time.Now(),
		TotalJobs:  len(jobs),
		Jobs:       jobs,
	}
	return json.MarshalIndent(bundle, "", "  ")
}

// ExportCSV generates a CSV representation of failed jobs.
func ExportCSV(jobs []adapters.Job) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write CSV header
	header := []string{"id", "queue", "queueType", "name", "errorReason", "attempts", "enqueuedAt", "payload"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	for _, j := range jobs {
		payloadStr := j.RawPayload
		if payloadStr == "" {
			b, _ := json.Marshal(j.Payload)
			payloadStr = string(b)
		}
		record := []string{
			j.ID,
			j.Queue,
			string(j.QueueType),
			j.Name,
			j.ErrorReason,
			fmt.Sprintf("%d", j.Attempts),
			j.EnqueuedAt.Format(time.RFC3339),
			payloadStr,
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}

// ImportResult summarizes the outcome of importing a DLQ bundle.
type ImportResult struct {
	TotalProcessed int      `json:"totalProcessed"`
	Succeeded      int      `json:"succeeded"`
	Failed         int      `json:"failed"`
	Errors         []string `json:"errors,omitempty"`
}

// ImportAndReplay parses a bundle and re-enqueues the jobs into their respective queues.
func ImportAndReplay(ctx context.Context, registry *adapters.Registry, rdb redis.UniversalClient, data []byte) (ImportResult, error) {
	var bundle DLQBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		// Try fallback array of jobs directly
		var rawJobs []adapters.Job
		if errArray := json.Unmarshal(data, &rawJobs); errArray != nil {
			return ImportResult{}, fmt.Errorf("invalid bundle JSON: %w", err)
		}
		bundle.Jobs = rawJobs
	}

	res := ImportResult{
		TotalProcessed: len(bundle.Jobs),
		Errors:         make([]string, 0),
	}

	for _, job := range bundle.Jobs {
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		default:
		}

		adapter, err := registry.Get(job.QueueType)
		if err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("job %s: unsupported queue type %s", job.ID, job.QueueType))
			continue
		}

		var payloadBytes []byte
		if job.RawPayload != "" {
			payloadBytes = []byte(job.RawPayload)
		} else if len(job.Payload) > 0 {
			payloadBytes, _ = json.Marshal(job.Payload)
		}

		err = adapter.RetryJob(ctx, rdb, job.Queue, job.ID, payloadBytes)
		if err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("job %s: %v", job.ID, err))
		} else {
			res.Succeeded++
		}
	}

	return res, nil
}

// BatchTransformRequest defines find-and-replace rules across job payloads in a cluster.
type BatchTransformRequest struct {
	QueueType       adapters.QueueType `json:"queueType"`
	Queue           string             `json:"queue"`
	JobIDs          []string           `json:"jobIds"`
	FindText        string             `json:"findText"`
	ReplaceText     string             `json:"replaceText"`
	ReplayAfterSave bool               `json:"replayAfterSave"`
	RateLimitPerSec int                `json:"rateLimitPerSec"`
}

// BatchTransform executes text replacement on raw payloads and optionally replays them.
func BatchTransform(ctx context.Context, registry *adapters.Registry, rdb redis.UniversalClient, req BatchTransformRequest) (ReplayResult, error) {
	start := time.Now()
	res := ReplayResult{
		TotalRequested: len(req.JobIDs),
		Errors:         make([]string, 0),
	}

	adapter, err := registry.Get(req.QueueType)
	if err != nil {
		return res, err
	}

	var interval time.Duration
	if req.RateLimitPerSec > 0 {
		interval = time.Second / time.Duration(req.RateLimitPerSec)
	}

	for _, jobID := range req.JobIDs {
		select {
		case <-ctx.Done():
			res.DurationMs = time.Since(start).Milliseconds()
			return res, ctx.Err()
		default:
		}

		job, err := adapter.GetJob(ctx, rdb, req.Queue, jobID)
		if err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("job %s: %v", jobID, err))
			continue
		}

		// Perform find and replace on raw payload
		raw := job.RawPayload
		if raw == "" {
			b, _ := json.Marshal(job.Payload)
			raw = string(b)
		}

		transformed := strings.ReplaceAll(raw, req.FindText, req.ReplaceText)

		if req.ReplayAfterSave {
			err = adapter.RetryJob(ctx, rdb, req.Queue, jobID, []byte(transformed))
			if err != nil {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("job %s: %v", jobID, err))
			} else {
				res.Succeeded++
			}
		} else {
			res.Succeeded++
		}

		if interval > 0 {
			time.Sleep(interval)
		}
	}

	res.DurationMs = time.Since(start).Milliseconds()
	return res, nil
}
