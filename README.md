# MicroBatcher

The MicroBatcher is a Go library for grouping individual tasks into smaller batches.

## Requirements

* [Go](https://golang.org) - developed with Go 1.22.1

## Instructions

The examples below describe a basic set-up to start working with the library.

### Download and install the package

```shell
go get github.com/peoxia/microbatcher
```

### Create a new MicroBatcher
```
microbatcher, err := New(batchProcessor, batchSize, batchIntervalMs, batchProcessorTimeout)
if err != nil
	return err
}
```

### Listen to the job results channel
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

### Start listening and processing jobs
```
microbatcher.ListenAndProcess()
```
### Submit jobs for processing
```
microbatcher.SubmitJob(Job)
```
### Shutdown the MicroBatcher
```
microbatcher.Shutdown()
```
