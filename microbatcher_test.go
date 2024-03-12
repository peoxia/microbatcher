package microbatcher

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMicroBatcher(t *testing.T) {
	batchProcessorSuccess := mockBatchProcessor{
		responseTime: 10 * time.Millisecond,
		response:     "success",
		err:          nil,
	}
	batchProcessorError := mockBatchProcessor{
		responseTime: 10 * time.Millisecond,
		response:     "",
		err:          errors.New("error in batch processor"),
	}

	batchProcessorDefaultTimeout := 500

	type args struct {
		batchProcessor        mockBatchProcessor
		batchIntervalMs       int
		batchSize             int
		batchProcessorTimeout int
	}
	type config struct {
		jobCount       int
		jobFrequencyMs int
	}
	type expected struct {
		batchesMin    int
		batchesMax    int
		batchResponse string
		batchErr      string
		Err           string
	}

	testTable := map[string]struct {
		Args     args
		Config   config
		Expected expected
	}{
		"Success - One job": {
			Args: args{
				batchProcessor:        batchProcessorSuccess,
				batchIntervalMs:       100,
				batchSize:             5,
				batchProcessorTimeout: batchProcessorDefaultTimeout,
			},
			Config: config{
				jobCount:       1,
				jobFrequencyMs: 5,
			},
			Expected: expected{
				batchesMin:    1,
				batchesMax:    1,
				batchResponse: "success",
			},
		},
		"Success - Batch fills up before batch interval elapses": {
			Args: args{
				batchProcessor:        batchProcessorSuccess,
				batchIntervalMs:       100,
				batchSize:             5,
				batchProcessorTimeout: batchProcessorDefaultTimeout,
			},
			Config: config{
				jobCount:       50,
				jobFrequencyMs: 2,
			},
			Expected: expected{
				batchesMin:    10,
				batchesMax:    10,
				batchResponse: "success",
			},
		},
		"Success - Batch interval elapses before batch fills up": {
			Args: args{
				batchProcessor:        batchProcessorSuccess,
				batchIntervalMs:       20,
				batchSize:             5,
				batchProcessorTimeout: batchProcessorDefaultTimeout,
			},
			Config: config{
				jobCount:       20,
				jobFrequencyMs: 10,
			},
			Expected: expected{
				batchesMin:    9,
				batchesMax:    10,
				batchResponse: "success",
			},
		},
		"Success - Batch interval elapses and batch fills up at the same time": {
			Args: args{
				batchProcessor:        batchProcessorSuccess,
				batchIntervalMs:       100,
				batchSize:             5,
				batchProcessorTimeout: batchProcessorDefaultTimeout,
			},
			Config: config{
				jobCount:       20,
				jobFrequencyMs: 20,
			},
			Expected: expected{
				batchesMin:    4,
				batchesMax:    5,
				batchResponse: "success",
			},
		},
		"Success - Batch processor response time is longer than batch interval": {
			Args: args{
				batchProcessor:        mockBatchProcessor{responseTime: 150 * time.Millisecond, response: "success", err: nil},
				batchIntervalMs:       100,
				batchSize:             5,
				batchProcessorTimeout: batchProcessorDefaultTimeout,
			},
			Config: config{
				jobCount:       25,
				jobFrequencyMs: 5,
			},
			Expected: expected{
				batchesMin:    5,
				batchesMax:    5,
				batchResponse: "success",
			},
		},
		"Error - Batch processor couldn't process batches": {
			Args: args{
				batchProcessor:        batchProcessorError,
				batchIntervalMs:       100,
				batchSize:             5,
				batchProcessorTimeout: batchProcessorDefaultTimeout,
			},
			Config: config{
				jobCount:       25,
				jobFrequencyMs: 5,
			},
			Expected: expected{
				batchesMin:    5,
				batchesMax:    5,
				batchResponse: "",
				batchErr:      "error processing batch: error in batch processor",
			},
		},
		"Error - Invalid batch size provided to start MicroBatcher": {
			Args: args{
				batchProcessor:        batchProcessorSuccess,
				batchIntervalMs:       100,
				batchSize:             -1,
				batchProcessorTimeout: batchProcessorDefaultTimeout,
			},
			Expected: expected{
				Err: "batch size must be greater than 0",
			},
		},
		"Error - Invalid batch interval provided to start MicroBatcher": {
			Args: args{
				batchProcessor:        batchProcessorSuccess,
				batchIntervalMs:       0,
				batchSize:             5,
				batchProcessorTimeout: batchProcessorDefaultTimeout,
			},
			Expected: expected{
				Err: "batch interval must be greater than 5ms",
			},
		},
		"Error - Invalid batch processor timeout provided to start MicroBatcher": {
			Args: args{
				batchProcessor:        batchProcessorSuccess,
				batchIntervalMs:       100,
				batchSize:             5,
				batchProcessorTimeout: 0,
			},
			Expected: expected{
				Err: "batch processor timeout must be greater than 0",
			},
		},
	}

	for tn, tc := range testTable {
		t.Run(tn, func(t *testing.T) {
			jobResults := make([]JobResult, 0)
			jobResultTimestamps := make([]time.Time, 0)
			jobsProcessed := 0

			var mutex sync.Mutex
			var wg sync.WaitGroup

			// Start microbatcher
			microbatcher, err := New(&tc.Args.batchProcessor, tc.Args.batchSize, tc.Args.batchIntervalMs, tc.Args.batchProcessorTimeout)
			if err != nil && tc.Expected.Err != "" {
				assert.EqualError(t, err, tc.Expected.Err)
				return
			}
			if err != nil && tc.Expected.Err == "" {
				t.Fatalf(err.Error())
			}
			jobResultCh := microbatcher.JobResults()

			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					jobResult, ok := <-jobResultCh
					if !ok {
						return
					}
					mutex.Lock()
					jobResults = append(jobResults, jobResult)
					mutex.Unlock()
				}
			}()

			microbatcher.ListenAndProcess()

			// Submit jobs
			for i := 0; i < tc.Config.jobCount; i++ {
				microbatcher.SubmitJob(Job{i, "job data"})
				time.Sleep(time.Duration(tc.Config.jobFrequencyMs) * time.Millisecond)
			}

			// Wait for processes to finish
			microbatcher.Shutdown()
			wg.Wait()

			// Batch processor calls
			assert.LessOrEqual(t, tc.Args.batchProcessor.calledTimes, tc.Expected.batchesMax, "Batch processor was called more times than maximum number of batches expected")
			assert.GreaterOrEqual(t, tc.Args.batchProcessor.calledTimes, tc.Expected.batchesMin, "Batch processor was called less times than minimum number of batches expected")

			// Job results
			assert.LessOrEqual(t, len(jobResults), tc.Expected.batchesMax, "Received more job results than maximum number of batches expected")
			assert.GreaterOrEqual(t, len(jobResults), tc.Expected.batchesMin, "Received less job results than minimum number of batches expected")

			for _, jobResult := range jobResults {
				if tc.Expected.batchErr != "" {
					assert.EqualError(t, jobResult.Err, tc.Expected.batchErr)
				} else {
					assert.NoError(t, jobResult.Err)
				}

				assert.LessOrEqual(t, len(jobResult.Batch), tc.Args.batchSize, "Received a batch larger than expected")

				jobsProcessed += len(jobResult.Batch)
				jobResultTimestamps = append(jobResultTimestamps, jobResult.Timestamp)
			}

			// Jobs submitted
			assert.Equal(t, tc.Config.jobCount, jobsProcessed, "Unexpected number of jobs processed")

			// Processing time
			expectedBatchIntervalMax := time.Duration(tc.Args.batchIntervalMs) + tc.Args.batchProcessor.responseTime + 100*time.Millisecond
			for i := range jobResultTimestamps {
				if i < 1 {
					continue
				}
				batchInterval := jobResultTimestamps[i].Sub(jobResultTimestamps[i-1])
				assert.LessOrEqual(t, batchInterval, expectedBatchIntervalMax)
			}
		})
	}
}

type mockBatchProcessor struct {
	responseTime time.Duration
	response     string
	err          error
	calledTimes  int
}

func (m *mockBatchProcessor) Process(_ context.Context, _ []Job) (interface{}, error) {
	m.calledTimes++
	time.Sleep(m.responseTime)
	return m.response, m.err
}
