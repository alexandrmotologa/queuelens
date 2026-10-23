package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// AlertSeverity defines the urgency of an operational notification.
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// AlertEvent represents a dispatchable incident notification.
type AlertEvent struct {
	Type      string                 `json:"type"` // stall_detected, error_spike, sla_breach
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Severity  AlertSeverity          `json:"severity"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// WebhookNotifier manages outbound webhook dispatches (Slack, Discord, generic HTTP).
type WebhookNotifier struct {
	mu         sync.RWMutex
	webhookURL string
	client     *http.Client
	lastAlerts map[string]time.Time
}

// NewWebhookNotifier instantiates an alert notifier.
func NewWebhookNotifier(webhookURL string) *WebhookNotifier {
	return &WebhookNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 5 * time.Second},
		lastAlerts: make(map[string]time.Time),
	}
}

// SetWebhookURL dynamically configures or updates the alert destination.
func (w *WebhookNotifier) SetWebhookURL(url string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.webhookURL = url
}

// GetWebhookURL returns the current webhook endpoint.
func (w *WebhookNotifier) GetWebhookURL() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.webhookURL
}

// SendAlert dispatches an alert payload with deduplication (max 1 alert per 60s per alert key).
func (w *WebhookNotifier) SendAlert(ctx context.Context, alert AlertEvent) error {
	w.mu.Lock()
	url := w.webhookURL
	if url == "" {
		w.mu.Unlock()
		return nil
	}

	// Throttle repeat alerts by type
	now := time.Now()
	if last, ok := w.lastAlerts[alert.Type]; ok && now.Sub(last) < 60*time.Second {
		w.mu.Unlock()
		return nil
	}
	w.lastAlerts[alert.Type] = now
	w.mu.Unlock()

	// Generic JSON payload with Slack and Discord compatibility text
	payload := map[string]interface{}{
		"text":      fmt.Sprintf("[%s] %s: %s", alert.Severity, alert.Title, alert.Message),
		"content":   fmt.Sprintf("[%s] %s: %s", alert.Severity, alert.Title, alert.Message),
		"alert":     alert,
		"timestamp": alert.Timestamp.Format(time.RFC3339),
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook responded with HTTP %d", resp.StatusCode)
	}

	return nil
}
