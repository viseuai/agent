// viseu-agent: run a Viseu AI Lab inference node.
//
// A self-contained foreground process: optionally spawns the local engine,
// heartbeats the models it serves to the gateway, stops cleanly on Ctrl+C.
// Configuration via flags, with environment variables as fallback.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/viseuai/agent/internal/beat"
	"github.com/viseuai/agent/internal/engine"
	"github.com/viseuai/agent/internal/meshsrv"
)

// meshStateExists reports whether a previous run already joined the mesh
// (state persists; the pre-auth key is only needed once).
func meshStateExists() bool {
	dir, err := os.UserConfigDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "viseu-agent", "tsnet"))
	return err == nil
}

func main() {
	hostname, _ := os.Hostname()

	gateway := flag.String("gateway", envOr("GATEWAY_URL", "https://api.viseuai.org"), "gateway base URL")
	key := flag.String("key", os.Getenv("NODE_KEY"), "node API key (vsk_..., role: node)")
	name := flag.String("name", envOr("NODE_NAME", hostname), "node name")
	engineURL := flag.String("engine-url", envOr("ENGINE_URL", "http://localhost:8090"), "local engine URL")
	meshKey := flag.String("mesh-key", os.Getenv("MESH_KEY"), "mesh pre-auth key (embedded node; first run only)")
	meshURL := flag.String("mesh-url", envOr("MESH_URL", "https://mesh.viseuai.org"), "mesh control server")
	meshPort := flag.Int("mesh-port", envInt("MESH_PORT", 8443), "port served on the mesh")
	advertise := flag.String("advertise-url", os.Getenv("ADVERTISE_URL"), "(advanced) mesh URL when running an external tailscale client")
	interval := flag.Int("interval", envInt("INTERVAL_SECONDS", 15), "heartbeat interval in seconds")
	engineCmd := flag.String("engine-cmd", os.Getenv("ENGINE_CMD"), "optional engine command to spawn and supervise")
	flag.Parse()

	if *key == "" {
		fmt.Fprintln(os.Stderr, "viseu-agent: -key is required (or NODE_KEY).")
		flag.Usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Default mode: embedded mesh node (no external installs). The agent
	// joins the mesh itself and proxies mesh traffic to the local engine.
	// Advanced mode: -advertise-url skips tsnet for hosts already on the
	// mesh via a system tailscale client.
	if *advertise == "" {
		engine, err := url.Parse(*engineURL)
		if err != nil {
			log.Fatalf("parsing engine url: %v", err)
		}
		stateExists := meshStateExists()
		if *meshKey == "" && !stateExists {
			fmt.Fprintln(os.Stderr, "viseu-agent: first run needs -mesh-key (or MESH_KEY) to join the mesh.")
			os.Exit(2)
		}
		log.Printf("joining mesh %s as %s", *meshURL, *name)
		adv, shutdown, err := meshsrv.Start(ctx, meshsrv.Config{
			ControlURL: *meshURL,
			AuthKey:    *meshKey,
			Hostname:   *name,
			Port:       *meshPort,
			Engine:     engine,
		})
		if err != nil {
			log.Fatalf("mesh: %v", err)
		}
		defer shutdown()
		*advertise = adv
		log.Printf("serving engine on the mesh at %s", adv)
	}

	var wg sync.WaitGroup
	if *engineCmd != "" {
		parts := strings.Fields(*engineCmd)
		sup := &engine.Supervisor{
			NewCmd: func() *exec.Cmd {
				c := exec.Command(parts[0], parts[1:]...)
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				return c
			},
			RestartDelay: 3 * time.Second,
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sup.Run(ctx)
		}()
	}

	a := &beat.Agent{
		GatewayURL:   *gateway,
		Key:          *key,
		Node:         *name,
		EngineURL:    *engineURL,
		AdvertiseURL: *advertise,
		Interval:     time.Duration(*interval) * time.Second,
	}
	log.Printf("agent %s: engine %s, advertising %s to %s every %s",
		a.Node, a.EngineURL, a.AdvertiseURL, a.GatewayURL, a.Interval)
	a.Run(ctx)

	wg.Wait()
	log.Print("agent stopped")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("%s must be an integer, got %q", key, v)
	}
	return n
}
