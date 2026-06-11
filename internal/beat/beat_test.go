package beat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func fakeEngine(t *testing.T, ids ...string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("engine path: %s", r.URL.Path)
		}
		type m struct {
			ID string `json:"id"`
		}
		var data []m
		for _, id := range ids {
			data = append(data, m{ID: id})
		}
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
	}))
	t.Cleanup(srv.Close)
	return srv
}

type gotBeat struct {
	auth string
	body struct {
		Node   string `json:"node"`
		Models []struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		} `json:"models"`
	}
}

func fakeGateway(t *testing.T, beats *atomic.Int32, last *gotBeat) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/nodes/heartbeat" || r.Method != http.MethodPost {
			t.Errorf("gateway got %s %s", r.Method, r.URL.Path)
		}
		last.auth = r.Header.Get("Authorization")
		json.NewDecoder(r.Body).Decode(&last.body)
		beats.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testAgent(engineURL, gatewayURL string) *Agent {
	return &Agent{
		GatewayURL:   gatewayURL,
		Key:          "vsk_test",
		Node:         "newton",
		EngineURL:    engineURL,
		AdvertiseURL: "http://100.64.0.3:8090",
		Interval:     10 * time.Millisecond,
	}
}

func TestOnceDiscoversAndReports(t *testing.T) {
	engine := fakeEngine(t, "qwen-3b", "qwen-7b")
	var beats atomic.Int32
	var last gotBeat
	gw := fakeGateway(t, &beats, &last)

	if err := testAgent(engine.URL, gw.URL).Once(context.Background()); err != nil {
		t.Fatalf("Once: %v", err)
	}

	if last.auth != "Bearer vsk_test" {
		t.Errorf("auth header: %q", last.auth)
	}
	if last.body.Node != "newton" {
		t.Errorf("node: %q", last.body.Node)
	}
	if len(last.body.Models) != 2 {
		t.Fatalf("models: %+v", last.body.Models)
	}
	if last.body.Models[0].ID != "qwen-3b" || last.body.Models[0].URL != "http://100.64.0.3:8090" {
		t.Errorf("model ad: %+v", last.body.Models[0])
	}
}

func TestOnceFailsWhenEngineDown(t *testing.T) {
	dead := httptest.NewServer(nil)
	dead.Close()
	var beats atomic.Int32
	var last gotBeat
	gw := fakeGateway(t, &beats, &last)

	if err := testAgent(dead.URL, gw.URL).Once(context.Background()); err == nil {
		t.Fatal("engine down must error (and not heartbeat)")
	}
	if beats.Load() != 0 {
		t.Error("no heartbeat should be sent when the engine is down")
	}
}

func TestOnceFailsOnGatewayRejection(t *testing.T) {
	engine := fakeEngine(t, "m")
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":{"type":"permission_error"}}`)
	}))
	t.Cleanup(gw.Close)

	if err := testAgent(engine.URL, gw.URL).Once(context.Background()); err == nil {
		t.Fatal("gateway 403 must surface as error")
	}
}

func TestRunBeatsRepeatedlyUntilCancelled(t *testing.T) {
	engine := fakeEngine(t, "m")
	var beats atomic.Int32
	var last gotBeat
	gw := fakeGateway(t, &beats, &last)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	testAgent(engine.URL, gw.URL).Run(ctx)

	if n := beats.Load(); n < 2 {
		t.Errorf("expected repeated heartbeats, got %d", n)
	}
}
