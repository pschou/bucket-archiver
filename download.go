package main

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/remeh/sizedwaitgroup"
)

// DownloadTask represents a file to download.
type DownloadTask struct {
	Size     int64
	Filename string
}

// DownloadedFile represents a file that has been downloaded.
type DownloadedFile struct {
	Size     int64
	Filename string

	TempFile string // Temporary file path if the file is large.
	Bytes    []byte // If the file is small, we can keep it in memory.
}

func putMemory(mem []byte) {
	// Function to return memory to the appropriate buffer pool based on size
	mem = mem[:cap(mem)]
	if len(mem) <= 32*1024 {
		bufPool32.Put(mem)
	} else {
		bufPool96.Put(mem)
	}
}

// Downloader listens for DownloadTask on tasksCh, downloads them, and sends DownloadedFile to doneCh.
func Downloader(ctx context.Context, tasksCh <-chan DownloadTask, doneCh chan<- DownloadedFile) {
	swg := sizedwaitgroup.New(16) // Limit to 16 concurrent downloads
	defer close(doneCh)           // Ensure doneCh is closed when the function exits

	for {
		select {
		case <-ctx.Done():
			break
		case task, ok := <-tasksCh:
			if !ok {
				return
			}
			parts := 1
			if task.Size > 8*1024*1024 {
				// If file is larger than 8MB, download in parts
				parts = 8
			}
			for i := 0; i < parts; i++ {
				swg.Add() // Add to the sized wait group for each part
			}

			go func(task DownloadTask, parts int) {
				defer func() {
					for i := 0; i < parts; i++ {
						swg.Done() // Mark the part as done
					}
				}()

				if task.Size <= 96*1024 { // If file is less than 32KB, download it in memory.
					// Use a buffer pool to reuse memory for small files
					// bufPool32 is for files <= 32KB, bufPool96 is for files <= 96KB
					// This avoids frequent memory allocations and deallocations.
					var mem []byte
					if task.Size <= 32*1024 {
						mem = bufPool32.Get().([]byte)
					} else {
						mem = bufPool96.Get().([]byte)
					}

					// If the file size is small enough, we can download it directly in memory
					n, err := downloadObjectToBuffer(ctx, srcBucket, task.Filename, mem)
					if err != nil {
						// Log the error and continue to the next file
						errCh <- &ErrorEvent{Size: task.Size, Filename: task.Filename,
							Err: fmt.Errorf("Error downloading object %s to memory: %v", task.Filename, err)}
						putMemory(mem)
						return
					}
					// Check if the number of bytes written matches the expected size
					if int64(n) != task.Size {
						errCh <- &ErrorEvent{Size: task.Size, Filename: task.Filename,
							Err: fmt.Errorf("Short write for object %s: expected %d, got %d", task.Filename, task.Size, n)}
						putMemory(mem)
						return
					}
					// Successfully downloaded the file to memory
					// Send the downloaded file to doneCh
					doneCh <- DownloadedFile{Size: task.Size, Filename: task.Filename,
						Bytes: mem[:n]} // Use the buffer directly as Filebytes
				} else {
					tempFilePath, err := downloadObjectInParts(ctx, srcBucket, task.Filename, task.Size, parts)
					if err != nil {
						// Log the error and continue to the next file
						errCh <- &ErrorEvent{Size: task.Size, Filename: task.Filename,
							Err: fmt.Errorf("Error downloading object %s to temporary file: %v", task.Filename, err)}
						return
					}
					// Successfully downloaded the file to a temporary file
					// Send the downloaded file to doneCh
					doneCh <- DownloadedFile{Size: task.Size, Filename: task.Filename, TempFile: tempFilePath}
				}
				atomic.AddInt64(&DownloadedFiles, 1)
			}(task, parts)
		}
	}
}
