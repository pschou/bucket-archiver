package main

import (
	"context"
	"log"
	"sync/atomic"
)

// Uploader listens for ArchiveFile on tasksCh, uploads them, and when the channel is closed sends a done
func Uploader(ctx context.Context, tasksCh <-chan ArchiveFile, doneCh chan<- struct{}) {
	log.Println("Starting uploader...")
	defer close(doneCh) // Ensure doneCh is closed when the function exits

	for {
		select {
		case <-ctx.Done():
			break
		case task, ok := <-tasksCh:
			if !ok {
				log.Println("Closing uploader...")
				return
			}

			if debug {
				log.Println("Sending file to upload", task.Filename)
			}
			if err := uploadFileInParts(ctx, dstBucket, task.Filename, task.Filename, 8); err != nil {
				log.Fatal(err)
			}
			atomic.AddInt64(&UploadedFiles, 1)
		}
	}
}
