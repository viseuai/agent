package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/viseuai/agent/internal/beat"
)

func main() {
	key := os.Getenv("NODE_KEY")
	if key == "" {
		log.Fatal("NODE_KEY is required (vsk_ key with the node role)")
	}
	advertise := os.Getenv("ADVERTISE_URL")
	if advertise == "" {
		log.Fatal("ADVERTISE_URL is required (mesh URL of the local engine, e.g. http://100.64.0.3:8090)")
	}

	hostname, _ := os.Hostname()
	a := &beat.Agent{
		GatewayURL:   envOr("GATEWAY_URL", "https://api.viseuai.org"),
		Key:          key,
		Node:         envOr("NODE_NAME", hostname),
		EngineURL:    envOr("ENGINE_URL", "http://localhost:8090"),
		AdvertiseURL: advertise,
		Interval:     time.Duration(envInt("INTERVAL_SECONDS", 15)) * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("agent %s: engine %s, advertising %s to %s every %s",
		a.Node, a.EngineURL, a.AdvertiseURL, a.GatewayURL, a.Interval)
	a.Run(ctx)
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
