package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/api"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/events"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/queue"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/store"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/worker"
)

func main() {
	port := flag.Int("port", 8080, "HTTP server listening port")
	redisURL := flag.String("redis", "", "Redis connection URL (e.g. redis://localhost:6379/0). If empty, uses in-memory mode.")
	maxQueueDepth := flag.Int64("max-queue-depth", 500, "Maximum allowed queue depth before backpressure 429")
	workers := flag.Int("workers", 4, "Number of embedded workers to start in standalone in-memory mode (set 0 to disable)")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var q queue.JobQueue
	var bus events.EventBus
	var st store.StateStore

	if *redisURL != "" {
		opt, err := redis.ParseURL(*redisURL)
		if err != nil {
			log.Fatalf("Invalid Redis URL: %v", err)
		}
		rdb := redis.NewClient(opt)

		pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
		if err := rdb.Ping(pingCtx).Err(); err != nil {
			pingCancel()
			log.Fatalf("Failed to connect to Redis at %s: %v", *redisURL, err)
		}
		pingCancel()
		log.Printf("Connected to Redis at %s", *redisURL)

		q = queue.NewRedisQueue(rdb, queue.DefaultRedisQueueKey)
		bus = events.NewRedisEventBus(rdb)
		st = store.NewRedisStore(rdb, 24*time.Hour)
	} else {
		log.Println("No Redis URL provided; running API gateway in In-Memory standalone mode.")
		q = queue.NewMemoryQueue(1000)
		bus = events.NewMemoryEventBus()
		st = store.NewMemoryStore()

		if *workers > 0 {
			log.Printf("Starting %d embedded worker routines for in-memory job processing...", *workers)
			poolCfg := worker.PoolConfig{
				Concurrency: *workers,
				WorkerID:    "embedded-worker-pool",
			}
			pool := worker.NewWorkerPool(poolCfg, q, bus, st)
			pool.Start(ctx)
			defer pool.Stop()
		}
	}

	serverCfg := api.ServerConfig{
		Addr:          fmt.Sprintf(":%d", *port),
		MaxQueueDepth: *maxQueueDepth,
	}

	srv := api.NewServer(serverCfg, q, bus, st)

	// Run server in goroutine
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server startup failed: %v", err)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	log.Println("Shutting down API Gateway...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	_ = q.Close()
	_ = bus.Close()
	_ = st.Close()
	log.Println("API Gateway shutdown complete.")
}
