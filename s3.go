package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/remeh/sizedwaitgroup"
)

var (
	region   string
	s3client *s3.Client

	uploadSWD            = sizedwaitgroup.New(2) // Limit concurrent uploads to 2
	s3Ready              sync.WaitGroup          // channel to signal when the S3 client is ready
	awscliLog            = log.New(os.Stderr, "awscli: ", log.LstdFlags)
	srcBucket, dstBucket string // Source and destination buckets
)

func init() {
	awscliLog.Println("Initializing S3 client...")
	s3RefreshTime, err := time.ParseDuration(Env("REFRESH", "20m", "The refresh interval for grabbing new AMI credentials"))
	if err != nil {
		awscliLog.Fatal("Invalid REFRESH duration:", err)
	}

	// Load environment variables for source and destination buckets and tarball key
	srcBucket = Env("SRC_BUCKET", "mySourceBucket", "The source S3 bucket name")
	dstBucket = Env("DST_BUCKET", "myDestinationBucket", "The destination S3 bucket name")

	s3Ready.Add(1) // Add to wait group to signal when the S3 client is ready
	go func() {
		defer s3Ready.Done() // Signal that the S3 client is ready

		/*sdkConfig, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			awscliLog.Fatal("Could not load default config,", err)
		}*/

		imdsClient := imds.New(imds.Options{})
		gro, err := imdsClient.GetRegion(context.TODO(), &imds.GetRegionInput{})
		if err != nil {
			awscliLog.Fatal("Could not get region property,", err)
		}

		iam, err := imdsClient.GetIAMInfo(context.TODO(), &imds.GetIAMInfoInput{})
		if err != nil {
			awscliLog.Fatal("Could not get IAM property,", err)
		}

		region = gro.Region
		awscliLog.Println("EC2 Environment:")
		awscliLog.Println("  AWS_REGION:", gro.Region)
		awscliLog.Println("  IMDS_ARN:", iam.IAMInfo.InstanceProfileArn)
		awscliLog.Println("  IMDS_ID:", iam.IAMInfo.InstanceProfileID)

		getConfig := func() error {
			// Get a credential provider from the configured role attached to the currently running EC2 instance
			provider := ec2rolecreds.New(func(o *ec2rolecreds.Options) {
				o.Client = imdsClient
			})

			// Construct a client, wrap the provider in a cache, and supply the region for the desired service
			s3client = s3.New(s3.Options{
				Credentials: aws.NewCredentialsCache(provider),
				Region:      region,
			})
			//fmt.Printf("config: %#v\n\n", sdkConfig)

			return nil
		}

		awscliLog.Println("Testing call to AWS...")
		if err := getConfig(); err != nil {
			awscliLog.Fatal("Error getting config:", err)
		}

		go func() {
			// Refresh credentials every 20 minutes to ensure low latency on requests
			// and recovery should the server not have a policy assigned to it yet.
			for {
				time.Sleep(s3RefreshTime)
				awscliLog.Printf("Pulling new creds for s3Client %#v\n", s3client)
				getConfig()
			}
		}()
		awscliLog.Println("S3 client initialized successfully")
	}()
}

func downloadObjectInParts(ctx context.Context, srcBucket string, key string, size int64, partCount int,
	currentObj, totalObj int, remainBytes int64) (string, error) {

	s3Ready.Wait()

	ext := filepath.Ext(key)
	if len(ext) == 0 {
		ext = ".tmp"
	}

	outFile, err := os.CreateTemp("", "s3obj-*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer outFile.Close()

	if err := outFile.Truncate(size); err != nil {
		return "", fmt.Errorf("failed to pre-allocate file: %w", err)
	}

	partSize := size / int64(partCount)
	var wg sync.WaitGroup
	errCh := make(chan error, partCount)

	var downloadedBytes int64 // atomic counter

	// Status output goroutine
	stopStatus := make(chan struct{})
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		var lastBytes int64
		var lastTime = time.Now()
		for {
			select {
			case <-stopStatus:
				return
			case <-ticker.C:
				curBytes := atomic.LoadInt64(&downloadedBytes)
				now := time.Now()
				elapsed := now.Sub(lastTime)
				fmt.Fprintf(os.Stderr, "%d/%d %s: %s/%s bytes (%s)\n", currentObj, totalObj, key,
					humanizeBytes(curBytes), humanizeBytes(size), humanizeRate(curBytes-lastBytes, elapsed))
				lastBytes = curBytes
				lastTime = now
			}
		}
	}()

	for i := 0; i < partCount; i++ {
		start := int64(i) * partSize
		end := start + partSize - 1
		if i == partCount-1 {
			end = size - 1
		}

		wg.Add(1)
		go func(partIdx int, start, end int64) {
			defer wg.Done()
			rangeHeader := fmt.Sprintf("bytes=%d-%d", start, end)
			getObj, err := s3client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(srcBucket),
				Key:    aws.String(key),
				Range:  aws.String(rangeHeader),
			})
			if err != nil {
				errCh <- fmt.Errorf("part %d: failed to get object: %w", partIdx, err)
				return
			}
			defer getObj.Body.Close()

			buf := make([]byte, 32*1024)
			offset := start
			for {
				n, readErr := getObj.Body.Read(buf)
				if n > 0 {
					_, writeErr := outFile.WriteAt(buf[:n], offset)
					if writeErr != nil {
						errCh <- fmt.Errorf("part %d: write error: %w", partIdx, writeErr)
						return
					}
					atomic.AddInt64(&downloadedBytes, int64(n))
					offset += int64(n)
				}
				if readErr == io.EOF {
					break
				}
				if readErr != nil {
					errCh <- fmt.Errorf("part %d: read error: %w", partIdx, readErr)
					return
				}
			}
		}(i, start, end)
	}

	wg.Wait()
	close(errCh)
	close(stopStatus)
	for e := range errCh {
		if e != nil {
			return "", e
		}
	}
	return outFile.Name(), nil
}

func downloadObjectToTempFile(ctx context.Context, srcBucket string, key string,
	currentObj, remainObj int, remainBytes int64) (string, error) {

	// Download an S3 object to a temporary file with the same extension as the S3 object
	s3Ready.Wait() // Wait for the S3 client to be ready
	getObj, err := s3client.GetObject(ctx, &s3.GetObjectInput{
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

	// Write the content of the S3 object to the temporary file using progressCp
	if _, err := progressCp(tmpFile, getObj.Body, *getObj.ContentLength, key, currentObj, remainObj, remainBytes); err != nil {
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Ensure the temporary file is closed and return its name
	return tmpFile.Name(), nil
}

func downloadObjectToBuffer(ctx context.Context, srcBucket string, key string, buf []byte) (int, error) {
	s3Ready.Wait() // Wait for the S3 client to be ready
	getObj, err := s3client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(srcBucket),
		Key:    &key,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to download object %s: %w", key, err)
	}
	defer getObj.Body.Close()

	n, err := io.ReadFull(getObj.Body, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return n, fmt.Errorf("failed to read object body: %w", err)
	}
	return n, nil
}

func processUpload(ctx context.Context, dstBucket string, filePath string) {
	s3Ready.Wait() // Wait for the S3 client to be ready
	uploadSWD.Add()
	go func(fileToUpload string) {
		defer uploadSWD.Done()
		if err := uploadFileToBucket(ctx, dstBucket, filePath, filePath); err != nil {
			awscliLog.Printf("Failed to upload %s: %v", filePath, err)
		} else {
			awscliLog.Printf("Uploaded %s to bucket %s", filePath, dstBucket)
		}
		os.Remove(fileToUpload) // Clean up the temporary file after upload
	}(filePath)
}

func uploadFileToBucket(ctx context.Context, dstBucket string, key string, filePath string) error {
	s3Ready.Wait() // Wait for the S3 client to be ready
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	_, err = s3client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(dstBucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String("application/octet-stream"), // Set appropriate content type
		Metadata:    virusScanMap,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to bucket %s with key %s: %w", dstBucket, key, err)
	}

	return nil
}
