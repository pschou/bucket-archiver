package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type MetaEntry struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

func loadMetadata(ctx context.Context, srcBucket string) (totalSize int64, objectCount int, err error) {
	log.Println("Loading metadata from S3 bucket:", srcBucket)
	// List objects in source bucket
	paginator := s3.NewListObjectsV2Paginator(s3client, &s3.ListObjectsV2Input{
		Bucket: aws.String(srcBucket),
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
