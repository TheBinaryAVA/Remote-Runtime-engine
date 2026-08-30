package worker

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/events"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/queue"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/store"
)

// PoolConfig holds worker pool configuration parameters.
type PoolConfig struct {
	Concurrency int
	PollTimeout time.Duration
	WorkerID    string
}

// WorkerPool manages multiple concurrent worker routines pulling jobs from the queue.
type WorkerPool struct {
	cfg        PoolConfig
	queue      queue.JobQueue
	eventBus   events.EventBus
	stateStore store.StateStore
	stopCh     chan struct{}
	wg         sync.WaitGroup
	activeJobs int64
	totalJobs  int64
	running    bool
	mu         sync.Mutex
}

// NewWorkerPool creates a new WorkerPool instance.
func NewWorkerPool(cfg PoolConfig, q queue.JobQueue, bus events.EventBus, st store.StateStore) *WorkerPool {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}
	if cfg.PollTimeout <= 0 {
		cfg.PollTimeout = 2 * time.Second
	}
	if cfg.WorkerID == "" {
		cfg.WorkerID = "worker-daemon"
	}

	return &WorkerPool{
		cfg:        cfg,
		queue:      q,
		eventBus:   bus,
		stateStore: st,
		stopCh:     make(chan struct{}),
	}
}

// Start launches the concurrent worker routines.
func (p *WorkerPool) Start(ctx context.Context) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	log.Printf("[%s] Starting worker pool with concurrency=%d", p.cfg.WorkerID, p.cfg.Concurrency)

	for i := 0; i < p.cfg.Concurrency; i++ {
		p.wg.Add(1)
		go p.workerLoop(ctx, i+1)
	}
}

func (p *WorkerPool) workerLoop(ctx context.Context, id int) {
	defer p.wg.Done()
	log.Printf("[%s] Worker #%d started and listening for jobs", p.cfg.WorkerID, id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] Worker #%d stopping (context cancelled)", p.cfg.WorkerID, id)
			return
		case <-p.stopCh:
			log.Printf("[%s] Worker #%d stopping (shutdown requested)", p.cfg.WorkerID, id)
			return
		default:
			job, err := p.queue.Dequeue(ctx, p.cfg.PollTimeout)
			if err != nil {
				if errors.Is(err, queue.ErrQueueEmpty) {
					continue
				}
				if errors.Is(err, queue.ErrQueueClosed) || ctx.Err() != nil {
					return
				}
				time.Sleep(200 * time.Millisecond)
				continue
			}

			atomic.AddInt64(&p.activeJobs, 1)
			atomic.AddInt64(&p.totalJobs, 1)

			jobStart := time.Now()
			log.Printf("[%s] Worker #%d processing submission %s (lang=%s, testcases=%d)",
				p.cfg.WorkerID, id, job.SubmissionID, job.Language, len(job.TestCases))

			if err := ProcessJob(ctx, job, p.eventBus, p.stateStore); err != nil {
				log.Printf("[%s] Worker #%d error on submission %s: %v", p.cfg.WorkerID, id, job.SubmissionID, err)
			} else {
				log.Printf("[%s] Worker #%d completed submission %s in %.2fms",
					p.cfg.WorkerID, id, job.SubmissionID, time.Since(jobStart).Seconds()*1000.0)
			}

			atomic.AddInt64(&p.activeJobs, -1)
		}
	}
}

// Stop initiates a graceful shutdown of all workers and waits for in-flight jobs to complete.
func (p *WorkerPool) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.stopCh)
	p.mu.Unlock()

	log.Printf("[%s] Waiting for active workers to complete...", p.cfg.WorkerID)
	p.wg.Wait()
	log.Printf("[%s] All workers exited gracefully", p.cfg.WorkerID)
}

// ActiveJobs returns the number of currently executing jobs.
func (p *WorkerPool) ActiveJobs() int64 {
	return atomic.LoadInt64(&p.activeJobs)
}

// TotalProcessed returns the lifetime count of processed submissions.
func (p *WorkerPool) TotalProcessed() int64 {
	return atomic.LoadInt64(&p.totalJobs)
}
