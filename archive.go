package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
)

var (
	archiveCount        = 0
	archiveTar          *tar.Writer
	archiveGzip         *gzip.Writer
	archiveFile         *os.File
	archiveBytesWritten int64

	doneArchiving = make(chan struct{})
)

// Archiver listens for ScannedFile on tasksCh, archives them, and sends to a bucket.
func Archiver(ctx context.Context, tasksCh <-chan ScannedFile) {

	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-tasksCh:
			if !ok {
				return
			}

			if archiveFile == nil {
				// Open the initial file
				OpenArchive()
			}

			if archiveBytesWritten > sizeCapLimit {
				// If the internal size is above the capacity limit, roll files
				CloseArchive()
				OpenArchive()
			}

			// Create a tar header for the file
			header := &tar.Header{
				Name: task.Filename,
				Size: task.Size,
				Mode: 0600, // Set file permissions
			}

			if err := archiveTar.WriteHeader(header); err != nil {
				log.Fatalf("failed to write tar header for %s: %v", task.Filename, err)
			}

			if len(task.Bytes) == 0 {
				continue
			}

			if task.TempFile == "" {
				if _, err := io.Copy(archiveTar, bytes.NewReader(task.Bytes)); err != nil {
					log.Fatalf("failed to write file %s to tar: %v", task.Filename, err)
				}
			} else {
				fh, err := os.Open(task.TempFile)
				if err != nil {
					log.Fatalf("failed to open temp file %s: %v", task.TempFile, err)
				}
				defer fh.Close()

				if _, err := io.Copy(archiveTar, fh); err != nil {
					log.Fatalf("failed to write file %s to tar: %v", task.Filename, err)
				}
			}
		}
	}
	if archiveFile != nil {
		CloseArchive()
	}

	close(doneArchiving)
}

func OpenArchive() {
	// Create a .tgz file on disk and prepare to write to it
	archiveCount++
	tgzFilePath := fmt.Sprintf("archive_%07d.tgz", archiveCount)
	archiveFile, err := os.Create(tgzFilePath)
	if err != nil {
		// No sense proceeding if the archives cannot be created
		log.Fatalf("failed to create tgz file: %v", err)
	}

	// Create a gzip writer and tar writer
	archiveGzip, err = gzip.NewWriterLevel(archiveFile, gzip.BestCompression)
	if err != nil {
		log.Fatalf("failed to create compressor for tgz file: %v", err)
	}
	archiveTar = tar.NewWriter(archiveGzip)
}

func CloseArchive() {
	if err := archiveTar.Close(); err != nil {
		log.Printf("failed to close tar writer: %v", err)
	}
	if err := archiveGzip.Close(); err != nil {
		log.Printf("failed to close gzip writer: %v", err)
	}
	if err := archiveFile.Close(); err != nil {
		log.Printf("failed to close tgz file: %v", err)
	}
}
