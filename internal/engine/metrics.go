package engine

import (
	"sync"
	"time"

	"github.com/alexandrmotologa/queuelens/internal/adapters"
)

// MetricBucket aggregates events within a 1-second timestamp slice.
type MetricBucket struct {
	Timestamp     int64 `json:"timestamp"` // epoch seconds
	Completed     int64 `json:"completed"`
	Failed        int64 `json:"failed"`
	Enqueued      int64 `json:"enqueued"`
	TotalDuration int64 `json:"totalDuration"`
	TotalWait     int64 `json:"totalWait"`
}

// MetricPoint is a snapshot item tailored for charting in the UI.
type MetricPoint struct {
	Time      string  `json:"time"`
	Timestamp int64   `json:"timestamp"`
	Throughput float64 `json:"throughput"`
	Completed int64   `json:"completed"`
	Failed    int64   `json:"failed"`
	Enqueued  int64   `json:"enqueued"`
	AvgWaitMs float64 `json:"avgWaitMs"`
	AvgExecMs float64 `json:"avgExecMs"`
}

// MetricsSummary represents the top-level throughput and health status.
type MetricsSummary struct {
	JobsPerSecond      float64       `json:"jobsPerSecond"`
	CompletedPerSec    float64       `json:"completedPerSec"`
	FailedPerSec       float64       `json:"failedPerSec"`
	ErrorRatePercent   float64       `json:"errorRatePercent"`
	AverageWaitMs      float64       `json:"averageWaitMs"`
	AverageExecMs      float64       `json:"averageExecMs"`
	TotalActiveQueues  int           `json:"totalActiveQueues"`
	TotalWaitingJobs   int64         `json:"totalWaitingJobs"`
	TotalActiveJobs    int64         `json:"totalActiveJobs"`
	TotalFailedJobs    int64         `json:"totalFailedJobs"`
	TimeToEmptySeconds int64         `json:"timeToEmptySeconds"`
	SLABreached        bool          `json:"slaBreached"`
	SLAMessage         string        `json:"slaMessage,omitempty"`
	History            []MetricPoint `json:"history"`
}

// MetricsEngine tracks sliding-window telemetry for queues.
type MetricsEngine struct {
	mu         sync.RWMutex
	windowSecs int
	buckets    map[int64]*MetricBucket
}

// NewMetricsEngine creates a sliding-window tracker over windowSecs (default 60).
func NewMetricsEngine(windowSecs int) *MetricsEngine {
	if windowSecs <= 0 {
		windowSecs = 60
	}
	return &MetricsEngine{
		windowSecs: windowSecs,
		buckets:    make(map[int64]*MetricBucket),
	}
}

// RecordEvent ingests a state change into the current second bucket.
func (m *MetricsEngine) RecordEvent(event JobEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sec := event.Timestamp.Unix()
	bucket, ok := m.buckets[sec]
	if !ok {
		bucket = &MetricBucket{Timestamp: sec}
		m.buckets[sec] = bucket
	}

	switch event.ToState {
	case adapters.StateCompleted:
		bucket.Completed++
		bucket.TotalDuration += event.DurationMs
	case adapters.StateFailed:
		bucket.Failed++
		bucket.TotalDuration += event.DurationMs
	case adapters.StateWaiting:
		bucket.Enqueued++
	case adapters.StateActive:
		bucket.TotalWait += event.WaitTimeMs
	}

	m.pruneOld(sec)
}

func (m *MetricsEngine) pruneOld(currentSec int64) {
	cutoff := currentSec - int64(m.windowSecs)
	for sec := range m.buckets {
		if sec < cutoff {
			delete(m.buckets, sec)
		}
	}
}

// GetSummary compiles current rates and 60-second chart data points.
func (m *MetricsEngine) GetSummary() MetricsSummary {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().Unix()
	m.pruneOld(now)

	var totalCompleted, totalFailed, totalEnqueued int64
	var totalExecMs, totalWaitMs int64

	history := make([]MetricPoint, 0, m.windowSecs)

	for i := m.windowSecs - 1; i >= 0; i-- {
		sec := now - int64(i)
		t := time.Unix(sec, 0)
		timeStr := t.Format("15:04:05")

		pt := MetricPoint{
			Time:      timeStr,
			Timestamp: sec,
		}

		if b, ok := m.buckets[sec]; ok {
			pt.Completed = b.Completed
			pt.Failed = b.Failed
			pt.Enqueued = b.Enqueued
			pt.Throughput = float64(b.Completed + b.Failed)
			if b.Completed+b.Failed > 0 {
				pt.AvgExecMs = float64(b.TotalDuration) / float64(b.Completed+b.Failed)
			}

			totalCompleted += b.Completed
			totalFailed += b.Failed
			totalEnqueued += b.Enqueued
			totalExecMs += b.TotalDuration
			totalWaitMs += b.TotalWait
		}

		history = append(history, pt)
	}

	windowFloat := float64(m.windowSecs)
	summary := MetricsSummary{
		JobsPerSecond:   float64(totalCompleted+totalFailed) / windowFloat,
		CompletedPerSec: float64(totalCompleted) / windowFloat,
		FailedPerSec:    float64(totalFailed) / windowFloat,
		History:         history,
	}

	totalFinished := totalCompleted + totalFailed
	if totalFinished > 0 {
		summary.ErrorRatePercent = (float64(totalFailed) / float64(totalFinished)) * 100.0
		summary.AverageExecMs = float64(totalExecMs) / float64(totalFinished)
	}
	if totalCompleted > 0 {
		summary.AverageWaitMs = float64(totalWaitMs) / float64(totalCompleted)
	}

	return summary
}

// ComputeSLA calculates queue drain estimations and flags SLA degradation.
func (m *MetricsEngine) ComputeSLA(summary *MetricsSummary, totalWaiting int64) {
	summary.TotalWaitingJobs = totalWaiting
	if summary.JobsPerSecond > 0.05 && totalWaiting > 0 {
		summary.TimeToEmptySeconds = int64(float64(totalWaiting) / summary.JobsPerSecond)
	}
	if summary.ErrorRatePercent > 10.0 {
		summary.SLABreached = true
		summary.SLAMessage = "Error rate exceeds 10% SLA threshold"
	} else if summary.TimeToEmptySeconds > 1800 {
		summary.SLABreached = true
		summary.SLAMessage = "Backlog drain time exceeds 30m SLA threshold"
	}
}
