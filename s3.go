package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func downloadObjectToTempFile(ctx context.Context, client *s3.Client, srcBucket string, key string) (string, error) {
	getObj, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(srcBucket),
		Key:    &key,
	})

	// Check if the object was successfully retrieved
	if err != nil {
		return "", fmt.Errorf("failed to download object %s: %w", key, err)
	}

	// Create a temporary file with the same extension as the S3 object
	// If the object has no extension, use .tmp
	ext := filepath.Ext(key)
	if len(ext) == 0 {
		ext = ".tmp"
	}

	// Create a temporary file in the system's temp directory
	tmpFile, err := os.CreateTemp("", "s3obj-*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	// Ensure the temporary file is closed after use
	defer tmpFile.Close()

	// Write the content of the S3 object to the temporary file
	if _, err := io.Copy(tmpFile, getObj.Body); err != nil {
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Ensure the temporary file is closed and return its name
	return tmpFile.Name(), nil
}

func uploadFileToBucket(ctx context.Context, client *s3.Client, dstBucket string, key string, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(dstBucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to bucket %s with key %s: %w", dstBucket, key, err)
	}

	return nil
}
