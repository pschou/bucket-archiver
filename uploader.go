package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync/atomic"
)

// Uploader listens for ArchiveFile on tasksCh, uploads them, and when the channel is closed sends a done
func Uploader(ctx context.Context, tasksCh <-chan ArchiveFile, doneCh chan<- struct{}) {
	log.Println("Starting uploader...")
	defer close(doneCh) // Ensure doneCh is closed when the function exits

	f, err := os.OpenFile("upload.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()

	for {
		select {
		case <-ctx.Done():
			break
		case task, ok := <-tasksCh:
			if !ok {
				Println("Closing uploader...")
				return
			}

			if debug {
				log.Println("Sending file to upload", task.Filename)
			}
			if err := uploadFileInParts(ctx, dstBucket, task.Filename, task.Filename, 8); err != nil {
				log.Fatal(err)
			}
			// Write successful uploads to log file
			for _, fileName := range task.Contents {
				fmt.Fprintln(f, fileName)
			}
			os.Remove(task.Filename)
			atomic.AddInt64(&UploadedFiles, 1)
		}
	}
}
