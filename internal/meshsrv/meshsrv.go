// Package meshsrv runs the node's mesh presence: an embedded userspace
// Tailscale node (tsnet) that listens on the association mesh and
// reverse-proxies inference requests to the local engine. The engine never
// listens beyond localhost; the volunteer installs nothing but the agent.
package meshsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"

	"tailscale.com/tsnet"
)

// EngineProxy forwards mesh requests to the local engine, preserving
// streaming (FlushInterval -1).
func EngineProxy(engine *url.URL) http.Handler {
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(engine)
			// Defense in depth (the gateway strips these too): no
			// credentials or browser headers reach the local engine.
			for _, h := range []string{"Authorization", "Cookie", "Origin", "Referer"} {
				pr.Out.Header.Del(h)
			}
		},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("engine proxy %s %s: %v", r.Method, r.URL.Path, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{
					"message": "The local engine is unavailable.",
					"type":    "api_error",
				},
			})
		},
	}
}

// Config for the embedded mesh node.
type Config struct {
	ControlURL string // https://mesh.viseuai.org
	AuthKey    string // headscale pre-auth key (first run only; state persists)
	Hostname   string
	Port       int      // mesh port to serve the engine on
	Engine     *url.URL // local engine base URL
}

// Start brings the embedded node up and serves the engine proxy on the
// mesh. Returns the URL peers use to reach this node.
func Start(ctx context.Context, cfg Config) (advertiseURL string, shutdown func(), err error) {
	stateDir, err := os.UserConfigDir()
	if err != nil {
		return "", nil, err
	}
	dir := filepath.Join(stateDir, "viseu-agent", "tsnet")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", nil, err
	}

	srv := &tsnet.Server{
		Dir:        dir,
		Hostname:   cfg.Hostname,
		ControlURL: cfg.ControlURL,
		AuthKey:    cfg.AuthKey,
		Logf:       func(string, ...any) {}, // tsnet is chatty; errors still surface via Up
	}

	status, err := srv.Up(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("joining the mesh: %w", err)
	}
	if len(status.TailscaleIPs) == 0 {
		srv.Close()
		return "", nil, fmt.Errorf("mesh up but no address assigned")
	}
	ip := status.TailscaleIPs[0]

	ln, err := srv.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		srv.Close()
		return "", nil, fmt.Errorf("listening on the mesh: %w", err)
	}

	go func() {
		if err := http.Serve(ln, EngineProxy(cfg.Engine)); err != nil {
			log.Printf("mesh server stopped: %v", err)
		}
	}()

	return fmt.Sprintf("http://%s:%d", ip, cfg.Port), func() { srv.Close() }, nil
}
