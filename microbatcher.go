package microbatcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// MicroBatcher represents a micro-batching library.
type MicroBatcher struct {
	// User config
	batchProcessor BatchProcessor
	batchSize      int
	batchInterval  time.Duration
	batchTimeout   time.Duration

	// Internal
	jobCh       chan Job
	jobResultCh chan JobResult
	waitGroup   sync.WaitGroup
	batchID     int
}

// Job represents a job to be processed.
type Job struct {
	ID   int
	Data interface{}
}

// JobResult represents the result of processing a job.
type JobResult struct {
	BatchID   int
	Batch     []Job
	Timestamp time.Time
	Response  interface{}
	Err       error
}

// BatchProcessor is an interface for processing batches of jobs.
type BatchProcessor interface {
	Process(ctx context.Context, batch []Job) (interface{}, error)
}

// New creates a new MicroBatcher instance with provided configurations.
//
// Parameters:
//   - batchSize: the maximum number of jobs in milliseconds to include in a batch.
//   - batchIntervalMs: the maximum time interval in milliseconds until the next batch is processed and
//     after the previous batch has been processed. The interval must be at least 5 ms.
//   - batchTimeoutMs: the timout for calls to BatchProcessor.
func New(batchProcessor BatchProcessor, batchSize, batchIntervalMs, batchTimeoutMs int) (*MicroBatcher, error) {
	if batchProcessor == nil {
		return nil, errors.New("batch processor is nil")
	}
	if batchSize < 1 {
		return nil, errors.New("batch size must be greater than 0")
	}
	if batchIntervalMs < 5 {
		return nil, errors.New("batch interval must be greater than 5ms")
	}
	if batchTimeoutMs < 1 {
		return nil, errors.New("batch processor timeout must be greater than 0")
	}
	return &MicroBatcher{
		batchProcessor: batchProcessor,
		batchSize:      batchSize,
		batchInterval:  time.Duration(batchIntervalMs) * time.Millisecond,
		batchTimeout:   time.Duration(batchTimeoutMs) * time.Millisecond,
		jobCh:          make(chan Job),
		jobResultCh:    make(chan JobResult),
		batchID:        0,
	}, nil
}

// JobResults return a read-only channel that receives job results. The channel is closed on shutdown of
// the Microbatcher.
func (mb *MicroBatcher) JobResults() <-chan JobResult {
	return mb.jobResultCh
}

// ListenAndProcess starts listening to submitted jobs and processing them in batches.
func (mb *MicroBatcher) ListenAndProcess() {
	mb.waitGroup.Add(1)
	go func() {
		defer mb.waitGroup.Done()
		mb.processJobs()
	}()
}

// SubmitJob submits a single job to the MicroBatcher.
func (mb *MicroBatcher) SubmitJob(job Job) {
	mb.jobCh <- job
}

// Shutdown gracefully shuts down the MicroBatcher and waits for all submitted jobs to be processed.
// The job result channel will be closed after the last job is processed.
func (mb *MicroBatcher) Shutdown() {
	close(mb.jobCh)

	mb.waitGroup.Wait()
}

func (mb *MicroBatcher) processJobs() {
	var batch []Job
	var mutex sync.Mutex
	timer := time.NewTimer(mb.batchInterval)
	defer timer.Stop()

	for {
		select {
		case job, ok := <-mb.jobCh:
			mutex.Lock()
			if !ok { // job channel is closed, process remaining batch and exit
				mb.processBatch(&batch, timer)
				close(mb.jobResultCh)
				return
			}
			batch = append(batch, job)
			if len(batch) >= mb.batchSize {
				mb.processBatch(&batch, timer)
			}
			mutex.Unlock()
		case <-timer.C:
			mutex.Lock()
			if len(batch) > 0 {
				mb.processBatch(&batch, timer)
			}
			mutex.Unlock()
		}
	}
}

func (mb *MicroBatcher) processBatch(batch *[]Job, timer *time.Timer) {
	if batch == nil || len(*batch) < 1 || timer == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), mb.batchTimeout)
	defer cancel()
	response, err := mb.batchProcessor.Process(ctx, *batch)
	if err != nil {
		err = fmt.Errorf("error processing batch: %w", err)
	}
	mb.jobResultCh <- JobResult{
		Batch:    *batch,
		BatchID:  mb.batchID,
		Response: response,
		Err:      err,
	}
	*batch = nil
	mb.batchID++

	// Clear any outdated timer alerts and reset the timer
	for len(timer.C) > 0 {
		<-timer.C
	}
	timer.Reset(mb.batchInterval)
}
