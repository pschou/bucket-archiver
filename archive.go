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

// DownloadTask represents a file to download.
type ArchiveFile struct {
	Filename string
	Contents []string
}

// Archiver listens for WorkFile on tasksCh, archives them, and sends to a bucket.
func Archiver(ctx context.Context, tasksCh <-chan *WorkFile, doneCh chan<- *ArchiveFile) {
	log.Println("Starting archiver...")
	defer close(doneCh)

	var tgzFile string
	var contents []string
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-tasksCh:
			if debug {
				log.Printf("Archiver task: %#v %v\n", task, ok)
			}

			if !ok {
				if tgzFile == "" {
					return
				}
				CloseArchive()
				FileContents := make([]string, len(contents))
				for i := range contents {
					FileContents[i] = contents[i]
				}
				doneCh <- &ArchiveFile{Filename: tgzFile, Contents: FileContents}
				contents = nil
				Println("Closing archiver...")
				return
			}

			if archiveFile == nil {
				// Open the initial file
				tgzFile = OpenArchive()
			}

			if debug {
				log.Println("Written", archiveBytesWritten, "Size Cap", sizeCapLimit)
			}
			if archiveBytesWritten > 0 && archiveBytesWritten+task.Size > sizeCapLimit {
				// If the internal size is above the capacity limit, roll files
				CloseArchive()
				FileContents := make([]string, len(contents))
				for i := range contents {
					FileContents[i] = contents[i]
				}
				doneCh <- &ArchiveFile{Filename: tgzFile, Contents: FileContents}
				contents = nil
				archiveBytesWritten = 0
				tgzFile = OpenArchive()
			}

			if debug {
				log.Println("Writing", task.Filename, "to tar with size", task.Size)
			}

			contents = append(contents, task.Filename)

			// Create a tar header for the file
			header := &tar.Header{
				Name: task.Filename,
				Size: task.Size,
				Mode: 0600, // Set file permissions
			}

			if err := archiveTar.WriteHeader(header); err != nil {
				log.Fatalf("failed to write tar header for %s: %v", task.Filename, err)
			}

			if task.Size == 0 {
				// Empty files don't need anything written, just the header
				continue
			}
			archiveBytesWritten += task.Size

			if task.TempFile == "" {
				if n, err := io.Copy(archiveTar, bytes.NewReader(task.Bytes)); err != nil {
					log.Fatalf("failed to write file %s to tar: %v", task.Filename, err)
				} else if debug {
					log.Println("Wrote", n, "bytes to tar")
				}
			} else {
				fh, err := os.Open(task.TempFile)
				if err != nil {
					log.Fatalf("failed to open temp file %s: %v", task.TempFile, err)
				}

				if n, err := io.Copy(archiveTar, fh); err != nil {
					log.Fatalf("failed to write file %s to tar: %v", task.Filename, err)
				} else if debug {
					log.Println("Wrote", n, "bytes to tar")
				}
				fh.Close()
				os.Remove(task.TempFile)
			}
			if debug {
				log.Println("Wrote", task.Filename, "to tar")
			}
		}
	}
}

func OpenArchive() string {
	// Create a .tgz file on disk and prepare to write to it
	archiveCount++
	tgzFilePath := fmt.Sprintf(ArchiveName, archiveCount)
	var err error
	archiveFile, err = os.Create(tgzFilePath)
	if err != nil {
		// No sense proceeding if the archives cannot be created
		log.Fatalf("failed to create tgz file: %v", err)
	}
	if debug {
		log.Println("created archive", tgzFilePath)
	}

	// Create a gzip writer and tar writer
	archiveGzip, err = gzip.NewWriterLevel(archiveFile, gzip.BestSpeed)
	if err != nil {
		log.Fatalf("failed to create compressor for tgz file: %v", err)
	}
	archiveTar = tar.NewWriter(archiveGzip)
	return tgzFilePath
}

func CloseArchive() {
	if archiveFile == nil {
		return
	}
	if err := archiveTar.Close(); err != nil {
		log.Printf("failed to close tar writer: %v", err)
	}
	if err := archiveGzip.Close(); err != nil {
		log.Printf("failed to close gzip writer: %v", err)
	}
	if err := archiveFile.Close(); err != nil {
		log.Printf("failed to close tgz file: %v", err)
	}
	archiveFile = nil
}
