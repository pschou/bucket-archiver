package main

// This program downloads objects from an S3 bucket, creates a tarball containing those objects
// and metadata, and uploads the tarball to another S3 bucket.

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

var (
	metadataFileName = "metadata.jsonl"
)

func main() {
	// Load environment variables for source and destination buckets and tarball key
	srcBucket := os.Getenv("SRC_BUCKET")
	log.Println("Source bucket:", srcBucket)
	dstBucket := os.Getenv("DST_BUCKET")
	log.Println("Destination bucket:", dstBucket)

	// Ensure source and destination buckets are set
	if srcBucket == "" || dstBucket == "" {
		log.Fatal("SRC_BUCKET and DST_BUCKET environment variables must be set")
	}

	ctx := context.Background()

	var totalSize int64
	var objectCount int

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
		totalSize = fileStats.Size
		objectCount = fileStats.Count
	} else if os.IsNotExist(err) {
		log.Printf("creating metadata file %q", metadataFileName)
		// Create metadata file if it doesn't exist
		totalSize, objectCount, err = loadMetadata(ctx, srcBucket)
		if err != nil {
			log.Fatalf("failed to load metadata: %v", err)
		}
	} else {
		log.Fatalf("error generating metadata file: %v", err)
	}
	log.Printf("Total objects: %d, Total size: %d bytes", objectCount, totalSize)

	// Open metadata file and parse each line for file size and name
	metadataFile, err := os.Open(metadataFileName)
	if err != nil {
		log.Fatalf("failed to open metadata file: %v", err)
	}
	defer metadataFile.Close()

	scanner := bufio.NewScanner(metadataFile)
	var (
		tgzFile      *os.File
		gzipWriter   *gzip.Writer
		tw           *tar.Writer
		archiveCount int
		tgzFilePath  = fmt.Sprintf("archive_%07d.tgz", archiveCount)

		uncompressedSize int64
	)

	for scanner.Scan() {
		if tgzFile == nil || uncompressedSize > 5*1024*1024*1024 {
			if tgzFile != nil {
				// If we have an existing tarball, close it before starting a new one
				fmt.Printf("Closing tarball %s with uncompressed size %d bytes\n", tgzFilePath, uncompressedSize)

				// Close the current tar writer and gzip writer
				if err := tw.Close(); err != nil {
					log.Fatalf("failed to close tar writer: %v", err)
				}
				if err := gzipWriter.Close(); err != nil {
					log.Fatalf("failed to close gzip writer: %v", err)
				}
				if err := tgzFile.Close(); err != nil {
					log.Fatalf("failed to close tgz file: %v", err)
				}
				archiveCount++       // Increment archive count for the next tarball
				uncompressedSize = 0 // Reset uncompressed size for the next tarball
			}
			// If the uncompressed size exceeds 5GB, create a new tarball

			// Create a .tgz file on disk and prepare to write to it
			tgzFilePath = fmt.Sprintf("archive_%07d.tgz", archiveCount)
			tgzFile, err = os.Create(tgzFilePath)
			if err != nil {
				log.Fatalf("failed to create tgz file: %v", err)
			}

			// Create a gzip writer and tar writer
			gzipWriter = gzip.NewWriter(tgzFile)
			tw = tar.NewWriter(gzipWriter)
		}

		// Parse each line as JSON to get file metadata
		// Assuming each line in metadata.jsonl is a JSON object with "name" and "size" fields
		var entry MetaEntry
		line := scanner.Bytes()
		if err := json.Unmarshal(line, &entry); err != nil {
			break // likely EOF or malformed line
		}
		fmt.Printf("Key: %s, Size: %d\n", entry.Key, entry.Size)

		tempFilePath, err := downloadObjectToTempFile(ctx, srcBucket, entry.Key)
		if err != nil {
			log.Fatalf("failed to download object %s: %v", entry.Key, err)
		}

		// Scan the file
		fmt.Printf("Scanning file: %s\n", tempFilePath)
		if _, err := ScanFile(tempFilePath); err != nil {
			log.Printf("Error scanning file %s: %v", entry.Key, err)
			os.Remove(tempFilePath) // Clean up temp file if scanning fails
			continue                // Skip this file if scanning fails
		}

		// Add metadata file to tarball
		metadataHeader := &tar.Header{
			Name: entry.Key,
			Mode: 0600,
			Size: entry.Size,
		}
		if err := tw.WriteHeader(metadataHeader); err != nil {
			log.Fatalf("failed to write metadata tar header: %v", err)
		}

		contents, err := os.Open(tempFilePath) // Open the temp file to read its content
		if err != nil {
			log.Fatalf("failed to open temp file %s: %v", tempFilePath, err)
		}

		if _, err = io.Copy(tw, contents); err != nil {
			log.Fatalf("failed to copy contents of %s to tarball: %v", tempFilePath, err)
		}
		uncompressedSize += entry.Size // Accumulate uncompressed size for the tarball

		contents.Close()        // Close the temp file after copying
		os.Remove(tempFilePath) // Clean up temp file after use
	}

	if tgzFile != nil {
		// If we have an existing tarball, close it before starting a new one
		fmt.Printf("Closing tarball archive_%07d.tgz with uncompressed size %d bytes\n", archiveCount, uncompressedSize)

		// Close the current tar writer and gzip writer
		if err := tw.Close(); err != nil {
			log.Fatalf("failed to close tar writer: %v", err)
		}
		if err := gzipWriter.Close(); err != nil {
			log.Fatalf("failed to close gzip writer: %v", err)
		}
		if err := tgzFile.Close(); err != nil {
			log.Fatalf("failed to close tgz file: %v", err)
		}

		if err := uploadFileToBucket(ctx, dstBucket, tgzFilePath, tgzFilePath); err != nil {
			log.Fatalf("failed to upload tgz file to S3: %v", err)
		}

		fmt.Printf("Uploaded %s to bucket %s\n", tgzFilePath, dstBucket)
		os.Remove(tgzFilePath)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("error reading metadata file: %v", err)
	}

}

func Env(env, def, usage string) string {
	fmt.Println("  #", usage)
	if e := os.Getenv(env); len(e) > 0 {
		fmt.Printf("  %s=%q\n", usage, env, e)
		return e
	}
	fmt.Printf("  %s=%q (default)\n", env, def)
	return def
}
