package ingestion

import (
	"context"
	"sync"
)

// ProcessChunksConcurrently runs chunk tasks in parallel with a bounded worker pool.
// It is intended for network-bound LLM enrichment steps (summaries, quiz seeds, Q/A extraction).
func ProcessChunksConcurrently(ctx context.Context, chunks []ChunkResult, workers int, task func(context.Context, ChunkResult) error) error {
	if len(chunks) == 0 {
		return nil
	}
	if workers <= 0 {
		workers = 1
	}

	jobs := make(chan ChunkResult)
	errCh := make(chan error, 1)

	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for chunk := range jobs {
			if ctx.Err() != nil {
				return
			}
			if err := task(ctx, chunk); err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
		}
	}

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}

	for _, chunk := range chunks {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case err := <-errCh:
			close(jobs)
			wg.Wait()
			return err
		case jobs <- chunk:
		}
	}
	close(jobs)
	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}
