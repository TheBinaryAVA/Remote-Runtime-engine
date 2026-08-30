package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/events"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/queue"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/store"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/worker"
)

func main() {
	redisURL := flag.String("redis", "", "Redis connection URL (e.g. redis://localhost:6379/0). If empty, uses in-memory mode.")
	concurrency := flag.Int("concurrency", 4, "Number of concurrent worker routines")
	workerID := flag.String("worker-id", "", "Unique worker instance identifier")

	flag.Parse()

	if *workerID == "" {
		host, _ := os.Hostname()
		*workerID = host + "-" + uuid.New().String()[:8]
	}

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

		// Test connection
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
		log.Println("No Redis URL provided; running in standalone In-Memory mode.")
		q = queue.NewMemoryQueue(1000)
		bus = events.NewMemoryEventBus()
		st = store.NewMemoryStore()
	}

	poolCfg := worker.PoolConfig{
		Concurrency: *concurrency,
		PollTimeout: 2 * time.Second,
		WorkerID:    *workerID,
	}

	pool := worker.NewWorkerPool(poolCfg, q, bus, st)
	pool.Start(ctx)

	// Graceful shutdown on SIGINT / SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	log.Println("Shutting down worker daemon...")
	cancel()
	pool.Stop()
	_ = q.Close()
	_ = bus.Close()
	_ = st.Close()
	log.Println("Worker daemon shutdown complete.")
}
