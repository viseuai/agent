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
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/viseuai/agent/internal/beat"
	"github.com/viseuai/agent/internal/engine"
)

func main() {
	hostname, _ := os.Hostname()

	gateway := flag.String("gateway", envOr("GATEWAY_URL", "https://api.viseuai.org"), "gateway base URL")
	key := flag.String("key", os.Getenv("NODE_KEY"), "node API key (vsk_..., role: node)")
	name := flag.String("name", envOr("NODE_NAME", hostname), "node name")
	engineURL := flag.String("engine-url", envOr("ENGINE_URL", "http://localhost:8090"), "local engine URL")
	advertise := flag.String("advertise-url", os.Getenv("ADVERTISE_URL"), "mesh URL the gateway uses to reach the engine")
	interval := flag.Int("interval", envInt("INTERVAL_SECONDS", 15), "heartbeat interval in seconds")
	engineCmd := flag.String("engine-cmd", os.Getenv("ENGINE_CMD"), "optional engine command to spawn and supervise")
	flag.Parse()

	if *key == "" || *advertise == "" {
		fmt.Fprintln(os.Stderr, "viseu-agent: -key and -advertise-url are required (or NODE_KEY / ADVERTISE_URL).")
		flag.Usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
