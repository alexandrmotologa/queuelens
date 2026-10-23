package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/alexandrmotologa/queuelens/internal/adapters"
	"github.com/alexandrmotologa/queuelens/internal/engine"
	"github.com/alexandrmotologa/queuelens/internal/server"
	"github.com/redis/go-redis/v9"
)

func TestServerAPI(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis error: %v", err)
	}
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer rdb.Close()

	registry := adapters.NewRegistry()
	broadcaster := engine.NewEventBroadcaster(100)
	metrics := engine.NewMetricsEngine(60)
	notifier := engine.NewWebhookNotifier("")
	stallAudit := engine.NewStallAuditor(registry, rdb, broadcaster, notifier)

	srv := server.NewServer(rdb, registry, broadcaster, metrics, stallAudit, notifier, s.Addr(), false)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	server.RegisterStaticHandlers(mux)

	// 1. Test /api/status
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on /api/status, got %d", w.Code)
	}

	// 2. Test /api/queues
	req = httptest.NewRequest(http.MethodGet, "/api/queues", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on /api/queues, got %d", w.Code)
	}

	// 3. Test /api/metrics
	req = httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on /api/metrics, got %d", w.Code)
	}

	// 4. Test /api/dlq/clusters
	req = httptest.NewRequest(http.MethodGet, "/api/dlq/clusters", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on /api/dlq/clusters, got %d", w.Code)
	}

	// 5. Test static fallback index.html
	req = httptest.NewRequest(http.MethodGet, "/stream", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on SPA fallback route /stream, got %d", w.Code)
	}

	// 6. Test DLQ export JSON
	req = httptest.NewRequest(http.MethodGet, "/api/dlq/export?format=json", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on /api/dlq/export, got %d", w.Code)
	}

	// 7. Test Alerts Config
	req = httptest.NewRequest(http.MethodGet, "/api/alerts/config", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on /api/alerts/config, got %d", w.Code)
	}
}
