package main

// This program downloads objects from an S3 bucket, creates a tarball containing those objects
// and metadata, and uploads the tarball to another S3 bucket.

import (
	"context"
	"log"
	"os"
)

var (
	metadataFileName = "metadata.jsonl"
	sizeCapLimit     = int64(1 * 1024 * 1024 * 1024) // 1 GB
	memoryOnlyScan   = make([]byte, 1*1024*1024)     // Placeholder for memory-only scan logic
)

func main() {
	// Parse SIZECAP environment variable if set, otherwise use default
	if sizeCapStr := os.Getenv("SIZECAP"); sizeCapStr != "" {
		parsed, err := parseByteSize(sizeCapStr)
		if err != nil {
			log.Fatalf("failed to parse SIZECAP: %v", err)
		}
		sizeCapLimit = parsed
		log.Printf("Using sizeCapLimit from SIZECAP env: %d bytes", sizeCapLimit)
	}
	log.Printf("Size cap limit for each tarball contents: %d bytes", sizeCapLimit)

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
			log.Fatalf("failed to read metadata file: %v", err)
		}
		TotalBytes = fileStats.Size
		TotalFiles = fileStats.Count
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
	log.Println("Starting to process files:", metadataFileName)

	var (
		toDownload      = make(chan DownloadTask, 10)
		downloadedFiles = make(chan DownloadedFile, 10)
		scannedFiles    = make(chan ScannedFile, 10)
		ArchiveFiles    = make(chan ArchiveFile, 2)
	)

	// Read the metadata and send it to the toDownload pipline
	ReadMetadata(ctx, toDownload)

	StartMetrics(ctx)

	// Consume the toDownload, download the file, and send to the downloaded pipeline
	go Downloader(ctx, toDownload, downloadedFiles)

	// Consume the downloaded, scan, and then send to the scannedFiles pipeline
	go Scanner(ctx, downloadedFiles, scannedFiles)

	// Consume the scanned files pipeline and put in archive
	go Archiver(ctx, scannedFiles, ArchiveFiles)

	StopMetrics()

	uploadSWD.Wait() // Wait for all uploads to finish
	log.Println("All uploads completed successfully.")
}
