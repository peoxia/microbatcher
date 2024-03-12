# MicroBatcher

The MicroBatcher is a Go library for grouping individual tasks into smaller batches.

## Requirements

* [Golang](https://golang.org) - developed with Golang 1.21.4

## Instructions

The examples below describe basic set-up to start working with the library.

### 1. Download and install the package

```shell
go get github.com/peoxia/microbatcher
```

### 2. Create a new MicroBatcher
```
microbatcher, err := New(batchProcessor, batchSize, batchIntervalMs, batchProcessorTimeout)
if err != nil
	return err
}
```

### 3. Listen to the job results channel
```
jobResultCh := microbatcher.JobResults()
go func() {
	for {
		jobResult, ok := <-jobResultCh
	    	if !ok {
			    return
			}
		// Process job result
	}
}()
```

### 4. Start listening and processing jobs
```
microbatcher.ListenAndProcess()
```
### 5. Submit jobs for processing
```
microbatcher.SubmitJob(Job)
```
### 6. Shutdown the MicroBatcher
```
microbatcher.Shutdown()
```