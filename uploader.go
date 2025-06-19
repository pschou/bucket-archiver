package main

import (
	"context"
	"log"
	"sync"
)

// Uploader listens for ArchiveFile on tasksCh, uploads them, and when the channel is closed sends a done
func Uploader(ctx context.Context, tasksCh <-chan ArchiveFile, doneCh chan<- struct{}) {
	log.Println("Starting uploader...")
	wg := sync.WaitGroup{} // Ensure all are done
	defer close(doneCh)    // Ensure doneCh is closed when the function exits

	for {
		select {
		case <-ctx.Done():
			break
		case task, ok := <-tasksCh:
			if !ok {
				wg.Wait()
				return
			}
			wg.Add(1)
			go func(task ArchiveFile) {
				defer wg.Done()
				uploadFileInParts(ctx, dstBucket, task.Filename, task.Filename, 8)
			}(task)
		}
	}
}
