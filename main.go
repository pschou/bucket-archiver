package main

// This program downloads objects from an S3 bucket, creates a tarball containing those objects
// and metadata, and uploads the tarball to another S3 bucket.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

var (
	metadataFileName = "metadata.jsonl"
	sizeCapLimit     int64
	debug            = Env("DEBUG", "", "Enable debugging") != ""
	ArchiveName      = Env("ARCHIVE_NAME", "archive_%07d.tgz", "Output template")
	version          = "1.0.0"
	scanningEnabled  = Env("DISABLE_SCANNER", "", "Disable the scanner") == ""
)

func main() {
	fmt.Printf("Starting bucket-archiver v%s: downloading, archiving, and uploading S3 objects.\n", version)

	// Parse SIZECAP environment variable if set, otherwise use default
	sizeCapStr := Env("SIZECAP", "2G", "Limit the size of the uncompressed archive payload")

	var err error
	sizeCapLimit, err = parseByteSize(sizeCapStr)
	if err != nil {
		log.Fatalf("failed to parse SIZECAP: %v", err)
	} else if sizeCapLimit < 100 {
		log.Fatalf("SIZECAP value %d is too small; must be at least 100 bytes", sizeCapLimit)
	}

	log.Println("Making pipeline channels.")
	var (
		toDownload      = make(chan DownloadTask, EnvInt("CHAN_TODO_DOWNLOAD", 10, "Buffer size for toDownload channel"))
		downloadedFiles = make(chan WorkFile, EnvInt("CHAN_DOWNLOADED_FILES", 20, "Buffer size for downloadedFiles channel"))
		scannedFiles    = make(chan WorkFile, EnvInt("CHAN_SCANNED_FILES", 10, "Buffer size for scannedFiles channel"))
		ArchiveFiles    = make(chan ArchiveFile, EnvInt("CHAN_ARCHIVE_FILES", 2, "Buffer size for ArchiveFiles channel"))
		Done            = make(chan struct{})
	)

	//log.Printf("Size cap limit for each tarball contents: %d bytes", sizeCapLimit)

	// Default context for processing
	ctx := context.Background()

	// Check if metadata file exists locally, if not, load metadata from S3
	//
	// If the metadata file exists, read it to get total size and object count
	// If it doesn't exist, create it by listing objects in the source bucket
	if _, err := os.Stat(metadataFileName); err == nil {
		log.Printf("metadata file %s already exists in the local filesystem", metadataFileName)

		// Read metadata from local file
		fileStats, err := ReadLastLineJSONStats(metadataFileName)
		if err != nil {
			log.Printf("failed to read metadata file: %v", err)
		} else {
			TotalBytes = fileStats.Size
			TotalFiles = fileStats.Count
		}
	} else if os.IsNotExist(err) {
		log.Printf("creating metadata file %q", metadataFileName)
		// Create metadata file if it doesn't exist
		TotalBytes, TotalFiles, err = loadMetadata(ctx, srcBucket)
		if err != nil {
			log.Fatalf("failed to load metadata: %v", err)
		}
	} else {
		log.Fatalf("error generating metadata file: %v", err)
	}
	log.Printf("Total objects: %d, Total size: %s", TotalFiles, humanizeBytes(TotalBytes))

	scanReady.Wait() // Wait for the ClamAV instance to be ready

	// Create a channel for error events to be handled by the error logger goroutine
	go func() {
		log.Println("Watching for errors...")
		f, err := os.OpenFile("error.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("failed to open err log file: %v", err)
		}
		defer f.Close()

		for errEvent := range errCh {
			data, err := json.Marshal(errEvent)
			if err != nil {
				log.Printf("failed to marshal error event: %v", err)
				continue
			}
			if _, err := fmt.Fprintf(f, "%s\n", data); err != nil {
				log.Printf("failed to write error event to file: %v", err)
			}
		}
	}()

	// Read the metadata and send it to the toDownload pipline
	go ReadMetadata(ctx, toDownload)

	StartMetrics(ctx)

	// Consume the toDownload, download the file, and send to the downloaded pipeline
	go Downloader(ctx, toDownload, downloadedFiles)

	if scanningEnabled {
		// Consume the downloaded, scan, and then send to the scannedFiles pipeline
		go Scanner(ctx, downloadedFiles, scannedFiles)

		// Consume the scanned files pipeline and put in archive
		go Archiver(ctx, scannedFiles, ArchiveFiles)
	} else {
		// Consume the scanned files pipeline and put in archive
		go Archiver(ctx, downloadedFiles, ArchiveFiles)
	}

	go Uploader(ctx, ArchiveFiles, Done)

	<-Done // Wait for all uploads to finish

	close(errCh) // Close error channel to ensure the logs are written to disk

	// Stop the metrics collection and clean up any resources
	StopMetrics()
	log.Println("All uploads completed successfully.")
	time.Sleep(time.Second)
}
