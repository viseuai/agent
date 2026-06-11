// Package beat implements the node agent's control loop: discover which
// models the local engine serves, report them to the gateway with an
// authenticated heartbeat, repeat. The gateway's registry forgets nodes
// whose heartbeats stop, so liveness IS the protocol.
package beat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Agent holds the node's configuration.
type Agent struct {
	GatewayURL   string        // https://api.viseuai.org
	Key          string        // vsk_ key with the node role
	Node         string        // node name (e.g. hostname)
	EngineURL    string        // local engine, e.g. http://localhost:8090
	AdvertiseURL string        // how the gateway reaches this engine (mesh URL)
	Interval     time.Duration // heartbeat period
	Client       *http.Client  // optional; defaults to a 10s-timeout client
}

type modelAd struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func (a *Agent) client() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// Once performs a single discover-and-report cycle.
func (a *Agent) Once(ctx context.Context) error {
	models, err := a.discover(ctx)
	if err != nil {
		return fmt.Errorf("discovering engine models: %w", err)
	}
	if err := a.send(ctx, models); err != nil {
		return fmt.Errorf("sending heartbeat: %w", err)
	}
	return nil
}

// Run heartbeats until the context is cancelled. Failures are logged and
// retried on the next tick; transient trouble must not kill the agent.
func (a *Agent) Run(ctx context.Context) {
	ticker := time.NewTicker(a.Interval)
	defer ticker.Stop()

	if err := a.Once(ctx); err != nil {
		log.Printf("heartbeat: %v", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.Once(ctx); err != nil {
				log.Printf("heartbeat: %v", err)
			}
		}
	}
}

func (a *Agent) discover(ctx context.Context) ([]modelAd, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.EngineURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	res, err := a.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("engine /v1/models: status %d", res.StatusCode)
	}

	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		return nil, err
	}
	ads := make([]modelAd, len(list.Data))
	for i, m := range list.Data {
		ads[i] = modelAd{ID: m.ID, URL: a.AdvertiseURL}
	}
	return ads, nil
}

func (a *Agent) send(ctx context.Context, models []modelAd) error {
	payload, err := json.Marshal(map[string]any{"node": a.Node, "models": models})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.GatewayURL+"/v1/nodes/heartbeat", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.Key)
	req.Header.Set("Content-Type", "application/json")

	res, err := a.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("gateway heartbeat: status %d: %s", res.StatusCode, body)
	}
	return nil
}
