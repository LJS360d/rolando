package utils

import (
	"fmt"
	"math"
	"rolando/internal/logger"
	"runtime"
	"sync"
)

const maxWorkers = 32

func ParallelTaskRunner[T any](input []T, loader func(T) error) error {
	// Determine the number of workers:
	// Use the minimum of maxWorkers and the number of chains,
	// capped at 2x the number of CPU cores for a good balance.
	inpuLen := len(input)
	numCPU := runtime.NumCPU()

	// Max number of concurrent goroutines to spawn.
	// Choose the smaller of our constant maxWorkers and a multiple of CPU cores.
	maxConcurrent := int(math.Min(float64(maxWorkers), float64(2*numCPU)))

	// Final number of workers is the lesser of the max concurrent or the total number of chains.
	numWorkers := int(math.Min(float64(maxConcurrent), float64(inpuLen)))
	logger.Debugf("Loader will use %d workers", numWorkers)
	if numWorkers == 0 {
		logger.Warnf("No data to load.")
		return nil
	}

	// Setup channels and sync primitives
	jobs := make(chan T, inpuLen)
	errCh := make(chan error, inpuLen)
	var wg sync.WaitGroup

	// Spawn worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each worker processes chains from the channel until it's closed
			for item := range jobs {
				// The chainsMap is a shared resource, so we must lock before writing
				err := loader(item)
				if err != nil {
					errCh <- err
				}
			}
		}()
	}

	// Send all items to the job channel
	for _, item := range input {
		jobs <- item
	}
	close(jobs) // Close the channel to signal workers no more jobs are coming

	// Wait for all workers to finish
	wg.Wait()

	close(errCh)

	// Check for any error
	errors := make([]error, 0)
	for err := range errCh {
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("%d errors occurred during parallel task execution: %v", len(errors), errors)
	}

	return nil
}
