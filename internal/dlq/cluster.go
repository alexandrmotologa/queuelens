package dlq

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/alexandrmotologa/queuelens/internal/adapters"
)

// Regular expressions to sanitize dynamic values from error messages.
var (
	uuidRegex   = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	ipRegex     = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?\b`)
	tsRegex     = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?`)
	idRegex     = regexp.MustCompile(`\b([a-zA-Z0-9_-]+[=:])?[a-zA-Z]*_\d+\b`)
	numRegex    = regexp.MustCompile(`\b\d{2,}\b`)
	spacesRegex = regexp.MustCompile(`\s+`)
)

// NormalizeSignature strips dynamic tokens to produce a deterministic root cause string.
func NormalizeSignature(raw string) string {
	if raw == "" {
		return "UnknownError: Empty failure reason"
	}

	// Use only the first 2-3 lines of error to avoid trace divergence
	lines := strings.Split(raw, "\n")
	sig := lines[0]
	if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
		sig += " | " + strings.TrimSpace(lines[1])
	}

	sig = uuidRegex.ReplaceAllString(sig, "<UUID>")
	sig = ipRegex.ReplaceAllString(sig, "<HOST:PORT>")
	sig = tsRegex.ReplaceAllString(sig, "<TIMESTAMP>")
	sig = idRegex.ReplaceAllString(sig, "<ID>")
	sig = numRegex.ReplaceAllString(sig, "<NUM>")
	sig = spacesRegex.ReplaceAllString(sig, " ")

	return strings.TrimSpace(sig)
}

// Fingerprint generates a short 12-char hex hash from the signature.
func Fingerprint(signature string) string {
	h := sha256.Sum256([]byte(signature))
	return hex.EncodeToString(h[:6])
}

// ErrorCluster groups failed jobs sharing the same error signature.
type ErrorCluster struct {
	ID               string                 `json:"id"`
	Signature        string                 `json:"signature"`
	SampleError      string                 `json:"sampleError"`
	SampleStackTrace string                 `json:"sampleStackTrace"`
	Count            int64                  `json:"count"`
	FirstSeen        time.Time              `json:"firstSeen"`
	LastSeen         time.Time              `json:"lastSeen"`
	Queues           []string               `json:"queues"`
	JobIDs           []string               `json:"jobIds"`
	SamplePayload    map[string]interface{} `json:"samplePayload"`
	SampleJobID      string                 `json:"sampleJobId"`
	SampleQueue      string                 `json:"sampleQueue"`
	SampleQueueType  adapters.QueueType     `json:"sampleQueueType"`
}

// ClusterJobs aggregates a slice of failed jobs into clusters.
func ClusterJobs(jobs []adapters.Job) []ErrorCluster {
	clusters := make(map[string]*ErrorCluster)
	queueSets := make(map[string]map[string]bool)

	for _, job := range jobs {
		reason := job.ErrorReason
		if reason == "" && job.StackTrace != "" {
			reason = job.StackTrace
		}

		sig := NormalizeSignature(reason)
		fp := Fingerprint(sig)

		c, exists := clusters[fp]
		if !exists {
			c = &ErrorCluster{
				ID:               fp,
				Signature:        sig,
				SampleError:      reason,
				SampleStackTrace: job.StackTrace,
				Count:            0,
				FirstSeen:        time.Now(),
				JobIDs:           make([]string, 0),
				SamplePayload:    job.Payload,
				SampleJobID:      job.ID,
				SampleQueue:      job.Queue,
				SampleQueueType:  job.QueueType,
			}
			clusters[fp] = c
			queueSets[fp] = make(map[string]bool)
		}

		c.Count++
		c.JobIDs = append(c.JobIDs, job.ID)
		queueSets[fp][job.Queue] = true

		if job.FinishedAt != nil {
			if job.FinishedAt.Before(c.FirstSeen) {
				c.FirstSeen = *job.FinishedAt
			}
			if job.FinishedAt.After(c.LastSeen) {
				c.LastSeen = *job.FinishedAt
			}
		}
	}

	result := make([]ErrorCluster, 0, len(clusters))
	for fp, c := range clusters {
		var qList []string
		for q := range queueSets[fp] {
			qList = append(qList, q)
		}
		sort.Strings(qList)
		c.Queues = qList

		if c.LastSeen.IsZero() {
			c.LastSeen = time.Now()
		}
		result = append(result, *c)
	}

	// Sort clusters by count descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	return result
}
