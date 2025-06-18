package main

// This program downloads objects from an S3 bucket, creates a tarball containing those objects
// and metadata, and uploads the tarball to another S3 bucket.

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	clamav "github.com/hexahigh/go-clamav"
)

var (
	metadataFileName = "metadata.jsonl"
	sizeCapLimit     = int64(1 * 1024 * 1024 * 1024) // 1 GB
	memoryOnlyScan   = make([]byte, 1*1024*1024)     // Placeholder for memory-only scan logic
)

func main() {
	// Load environment variables for source and destination buckets and tarball key
	srcBucket := os.Getenv("SRC_BUCKET")
	log.Println("Source bucket:", srcBucket)
	dstBucket := os.Getenv("DST_BUCKET")
	log.Println("Destination bucket:", dstBucket)

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

		readSize         int64
		uncompressedSize int64
	)

	log.Println("Starting to process metadata file:", metadataFileName)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if tgzFile == nil || uncompressedSize > sizeCapLimit {
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
			log.Printf("failed to unmarshal line %q: %v", line, err)
			break // likely EOF or malformed line
		}
		if entry.Key == "" {
			break
		}

		percent := float64(readSize) / float64(totalSize) * 100
		fmt.Printf("%d/%d %.2f%%: %s\n", lineNumber, objectCount, percent, entry.Key)
		//fmt.Printf("Key: %s, Size: %d\n", entry.Key, entry.Size)

		var tempFilePath string
		var tempFileMem []byte

		if entry.Size <= int64(cap(memoryOnlyScan)) {
			// If the file size is small enough, we can scan it directly in memory
			// This is a placeholder for memory-only scan logic
			// fmt.Printf("Scanning %s in memory\n", entry.Key)
			n, err := downloadObjectToBuffer(ctx, srcBucket, entry.Key, memoryOnlyScan)
			if err != nil {
				log.Printf("Error downloading object %s to memory: %v", entry.Key, err)
				continue // Skip this file if download fails
			}
			tempFileMem = memoryOnlyScan[:n] // Use the downloaded bytes

			fmem := clamav.OpenMemory(tempFileMem)
			if fmem == nil {
				log.Printf("Failed to open memory for scanning %s", entry.Key)
				continue // Skip this file if memory scan fails
			}
			// Scan the file
			//fmt.Printf("Scanning file: %s\n", tempFilePath)
			_, virusName, err := clamavInstance.ScanMapCB(fmem, entry.Key, context.Background())
			//clamav.CloseMemory(fmem) // Clean up memory after scanning

			if virusName != "" {
				//log.Printf("Virus found in %q: %s\n", filePath, virusName)
				// If a virus is found, return an error with the virus name
				// and the file path for clarity.}
				log.Printf("In %q found %q", entry.Key, virusName)
				continue
			} else if err != nil {
				//log.Println("Error scanning file:", err)
				log.Printf("In %q error %v", entry.Key, err)
				continue
			}
		} else {
			// For larger files, download them to a temporary file
			tempFilePath, err = downloadObjectToTempFile(ctx, srcBucket, entry.Key)
			if err != nil {
				log.Fatalf("failed to download object %s: %v", entry.Key, err)
			}

			// Scan the file
			//fmt.Printf("Scanning file: %s\n", tempFilePath)
			_, virusName, err := clamavInstance.ScanFile(tempFilePath)
			if virusName != "" {
				//log.Printf("Virus found in %q: %s\n", filePath, virusName)
				// If a virus is found, return an error with the virus name
				// and the file path for clarity.}
				log.Printf("In %q found %q", entry.Key, virusName)
				os.Remove(tempFilePath) // Clean up temp file if scanning fails
				continue
			} else if err != nil {
				//log.Println("Error scanning file:", err)
				log.Printf("In %q error %v", entry.Key, err)
				os.Remove(tempFilePath) // Clean up temp file if scanning fails
				continue
			}
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

		if tempFilePath == "" {
			// If we scanned the file in memory, write the memory buffer to the tarball
			if _, err := io.Copy(tw, bytes.NewReader(tempFileMem)); err != nil {
				log.Fatalf("failed to copy contents of %s to tarball: %v", tempFilePath, err)
			} else {
				//fmt.Printf("Copied %d bytes from %s to tarball\n", n, tempFilePath)
			}

			fmt.Printf("Wrote %d bytes from memory buffer to tarball\n", len(tempFileMem))
		} else {
			// If we downloaded the file to a temporary file, read its contents
			contents, err := os.Open(tempFilePath) // Open the temp file to read its content
			if err != nil {
				log.Fatalf("failed to open temp file %s: %v", tempFilePath, err)
			}

			if _, err := io.Copy(tw, contents); err != nil {
				log.Fatalf("failed to copy contents of %s to tarball: %v", tempFilePath, err)
			} else {
				//fmt.Printf("Copied %d bytes from %s to tarball\n", n, tempFilePath)
			}
			uncompressedSize += entry.Size // Accumulate uncompressed size for the tarball
			readSize += entry.Size         // Accumulate total size for all files processed

			contents.Close()        // Close the temp file after copying
			os.Remove(tempFilePath) // Clean up temp file after use
		}
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

// parseByteSize parses a human-readable byte size string (e.g., "1GB", "500MB", "100K") into int64 bytes.
func parseByteSize(s string) (int64, error) {
	var size int64
	var unit string
	n, err := fmt.Sscanf(s, "%d%s", &size, &unit)
	if n < 1 || err != nil {
		return 0, fmt.Errorf("invalid size format: %q", s)
	}
	switch unit {
	case "", "B", "b":
		return size, nil
	case "K", "KB", "k", "kb":
		return size * 1024, nil
	case "M", "MB", "m", "mb":
		return size * 1024 * 1024, nil
	case "G", "GB", "g", "gb":
		return size * 1024 * 1024 * 1024, nil
	case "T", "TB", "t", "tb":
		return size * 1024 * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unknown size unit: %q", unit)
	}
}
