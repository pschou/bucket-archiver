package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type MetaEntry struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

var (
	subSetFiles = Env("SUBSET", "", "Subset the files by START:STRIDE or START:STRIDE:END")
	skipFiles   = make(map[string]struct{})
)

func loadMetadata(ctx context.Context, srcBucket string) (totalSize, objectCount int64, err error) {
	s3Ready.Wait() // Wait for the S3 client to be ready
	log.Println("Loading metadata from S3 bucket:", srcBucket)

	prefixFilter := Env("PREFIX_FILTER", "", "Bucket prefix selector")
	var prefix, slash *string
	if prefixFilter != "" {
		prefix = aws.String(prefixFilter)
	}
	if Env("PREFIX_DELIM", "", "Use delimitor") != "" {
		slash = aws.String("/")
	}

	// List objects in source bucket
	paginator := s3.NewListObjectsV2Paginator(s3client, &s3.ListObjectsV2Input{
		Bucket:    aws.String(srcBucket),
		Prefix:    prefix,
		Delimiter: slash,
	})

	// Open metadata.json for writing
	metadataFile, err := os.Create(metadataFileName)
	if err != nil {
		log.Fatalf("failed to create metadata.json: %v", err)
	}

	// Use a buffered writer for better performance
	metadataBuf := bufio.NewWriter(metadataFile)

	// Ensure the metadata file is closed and flushed properly
	defer func() {
		log.Println("Writing out metadata file")
		if err := metadataBuf.Flush(); err != nil {
			log.Fatalln("Error writing metadata,", err)
		}
		if err := metadataFile.Close(); err != nil {
			log.Fatalln("Error closing metadata file,", err)
		}
	}()

	// Iterate through all pages of objects
	for paginator.HasMorePages() {
		// Get the next page of objects
		page, err := paginator.NextPage(ctx)
		if err != nil {
			log.Fatalf("failed to list objects: %v", err)
		}

		for _, obj := range page.Contents {
			// Prepare metadata file content
			if obj.Key == nil || obj.Size == nil {
				continue
			}

			// Count objects and accumulate total size
			objectCount++
			totalSize += *obj.Size

			// Write metadata line
			// Format: {"name":"object_key","size":object_size}
			dat, _ := json.Marshal(MetaEntry{Key: *obj.Key, Size: *obj.Size})
			metadataBuf.Write(dat)
			metadataBuf.WriteByte('\n')
		}
	}

	// Write summary metadata
	summaryLine := fmt.Sprintf(`{"total_objects":%d,"total_size":%d}`+"\n", objectCount, totalSize)
	metadataBuf.WriteString(summaryLine)
	log.Printf("Metadata written: %d objects, total size %d bytes\n", objectCount, totalSize)

	log.Println("Metadata file created successfully:", metadataFileName)
	// Print summary
	log.Printf("Total objects: %d, Total size: %d bytes\n", objectCount, totalSize)
	if objectCount == 0 {
		log.Println("No objects found in the source bucket.")
	} else {
		log.Printf("Metadata file %s created with %d objects and total size %d bytes.\n", metadataFileName, objectCount, totalSize)
	}

	return
}

func ReadMetadata(ctx context.Context, doFiles chan<- *DownloadTask) {

	f, err := os.Open("upload.log")
	if err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			skipFiles[strings.TrimSpace(scanner.Text())] = struct{}{}
		}
		f.Close()
	}

	log.Println("Reading in", metadataFileName, "for processing...")
	defer close(doFiles)

	// Open metadata file and parse each line for file size and name
	metadataFile, err := os.Open(metadataFileName)
	if err != nil {
		log.Fatalf("failed to open metadata file: %v", err)
	}
	defer metadataFile.Close()

	var start, stride, end int
	if subSetFiles == "" {
		stride = 1
		end = -1
	} else {
		if n, err := fmt.Sscanf(subSetFiles, "%d:%d:%d", &start, &stride, &end); err == nil && n == 3 {
			// All fields are provided, NOOP
		} else if n, err = fmt.Sscanf(subSetFiles, "%d:%d", &start, &stride); err == nil && n == 2 {
			// Try START:STRIDE
			end = -1 // Use -1 or another sentinel value to indicate "no end"
		}

		// First pass to do size accounting with the stride accounting
		TotalBytes = 0
		TotalFiles = 0

		scanner := bufio.NewScanner(metadataFile)
		lineNumber := 0
		strider := 0
		for scanner.Scan() {
			if debug {
				log.Println("scanned:", scanner.Text())
			}
			lineNumber++
			if start > 0 {
				start--
				continue
			}
			if end != -1 && lineNumber > end {
				break
			}
			if stride > 1 {
				strider = (strider + 1) % stride
				if strider != 1 {
					continue
				}
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
			atomic.AddInt64(&TotalBytes, entry.Size)
			atomic.AddInt64(&TotalFiles, 1)
		}
	}

	scanner := bufio.NewScanner(metadataFile)
	if debug {
		log.Println("start:", start, "stride:", stride, "end:", end)
	}

	lineNumber := 0
	strider := 0
	for scanner.Scan() {
		if debug {
			log.Println("scanned:", scanner.Text())
		}
		lineNumber++
		if start > 0 {
			start--
			continue
		}
		if end != -1 && lineNumber > end {
			break
		}
		if stride > 1 {
			strider = (strider + 1) % stride
			if strider != 1 {
				continue
			}
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
		if _, ok := skipFiles[entry.Key]; ok {
			if debug {
				log.Printf("skipping dup: %#v\n", entry)
			}
			atomic.AddInt64(&TotalBytes, -entry.Size)
			atomic.AddInt64(&TotalFiles, -1)
			continue
		}

		if debug {
			log.Println("sending:", scanner.Text())
		}

		if debug {
			log.Printf("sent task: %#v\n", entry)
		}
		doFiles <- &DownloadTask{Filename: entry.Key, Size: entry.Size}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("error reading metadata file: %v", err)
	}
}
