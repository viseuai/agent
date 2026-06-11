package meshsrv

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func target(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestProxiesRequestsToLocalEngine(t *testing.T) {
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Fprintf(w, "%s %s %s", r.Method, r.URL.Path, body)
	}))
	defer engine.Close()

	h := EngineProxy(target(t, engine.URL))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if got, want := rec.Body.String(), `POST /v1/chat/completions {"model":"m"}`; got != want {
		t.Errorf("proxied request: got %q, want %q", got, want)
	}
}

func TestStreamingFlushesThrough(t *testing.T) {
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"a\":1}\n\n")
		fl.Flush()
		fmt.Fprint(w, "data: [DONE]\n\n")
		fl.Flush()
	}))
	defer engine.Close()

	h := EngineProxy(target(t, engine.URL))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "data: [DONE]") {
		t.Errorf("stream incomplete: %s", rec.Body)
	}
	if !rec.Flushed {
		t.Error("response was buffered; streaming requires flush-through")
	}
}

func TestEngineDownIs502(t *testing.T) {
	dead := httptest.NewServer(nil)
	dead.Close()

	h := EngineProxy(target(t, dead.URL))
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status: got %d, want 502", rec.Code)
	}
}
