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
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var (
	region   string
	s3client *s3.Client

	s3Ready              sync.WaitGroup // channel to signal when the S3 client is ready
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

	// Ensure source and destination buckets are set
	if srcBucket == "" || dstBucket == "" {
		awscliLog.Fatal("SRC_BUCKET and DST_BUCKET environment variables must be set")
	}

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

func downloadObjectInParts(ctx context.Context, srcBucket string, key string, size int64, partCount int) (string, error) {

	s3Ready.Wait()

	ext := filepath.Ext(key)
	if len(ext) == 0 {
		ext = ".tmp"
	}

	outFile, err := os.CreateTemp("", "s3obj-*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	if err := outFile.Truncate(size); err != nil {
		outFile.Close()
		os.Remove(outFile.Name())
		return "", fmt.Errorf("failed to pre-allocate file: %w", err)
	}
	defer outFile.Close()

	var (
		partSize = size / int64(partCount)
		wg       sync.WaitGroup
		errCh    = make(chan error, partCount)
		proceed  = true
	)

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
				proceed = false
				// If we encounter an error, we stop processing and report the error
				errCh <- fmt.Errorf("part %d: failed to get object: %w", partIdx, err)
				return
			}
			defer getObj.Body.Close()

			buf := bufPool32.Get().([]byte)
			defer bufPool32.Put(buf)
			offset := start
			for proceed {
				n, readErr := getObj.Body.Read(buf)
				if n > 0 {
					_, writeErr := outFile.WriteAt(buf[:n], offset)
					if writeErr != nil {
						proceed = false
						// If we encounter a write error, we stop writing and report the error
						errCh <- fmt.Errorf("part %d: write error: %w", partIdx, writeErr)
						return
					}
					atomic.AddInt64(&DownloadedBytes, int64(n))
					offset += int64(n)
				}
				if readErr == io.EOF {
					break
				}
				if readErr != nil {
					proceed = false
					// If we encounter an error, we stop reading and report the error
					errCh <- fmt.Errorf("part %d: read error: %w", partIdx, readErr)
					return
				}
			}
		}(i, start, end)
	}

	wg.Wait()
	close(errCh)
	for e := range errCh {
		if e != nil {
			proceed = false
			outFile.Close()
			os.Remove(outFile.Name())
			return "", e
		}
	}

	return outFile.Name(), nil
}

func downloadObjectToBuffer(ctx context.Context, srcBucket string, key string, localBuf []byte) (int, error) {
	s3Ready.Wait() // Wait for the S3 client to be ready
	getObj, err := s3client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(srcBucket),
		Key:    &key,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to download object %s: %w", key, err)
	}
	defer getObj.Body.Close()

	var total int

	for len(localBuf) > 0 {
		n, readErr := getObj.Body.Read(localBuf)
		if n > 0 {
			localBuf = localBuf[n:] // Reduce the buffer size
			atomic.AddInt64(&DownloadedBytes, int64(n))
			total += n
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return total, fmt.Errorf("failed to read object body: %w", readErr)
		}
	}
	return total, nil
}

func uploadFileInParts(ctx context.Context, dstBucket, key, filePath string, partCount int) error {
	s3Ready.Wait() // Wait for the S3 client to be ready

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}
	size := info.Size()
	if partCount <= 0 || size == 0 {
		return fmt.Errorf("invalid partCount or file size")
	}

	partSize := size / int64(partCount)
	if size%int64(partCount) != 0 {
		partSize++
	}

	createResp, err := s3client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(dstBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to initiate multipart upload: %w", err)
	}

	var (
		wg       sync.WaitGroup
		errCh    = make(chan error, partCount)
		etags    = make([]s3types.CompletedPart, partCount)
		uploadID = *createResp.UploadId
	)

	for i := 0; i < partCount; i++ {
		start := int64(i) * partSize
		end := start + partSize
		if end > size {
			end = size
		}
		partNum := int32(i + 1)
		partLen := end - start

		wg.Add(1)
		go func(idx int, partNum int32, start, partLen int64) {
			defer wg.Done()
			upResp, err := s3client.UploadPart(ctx, &s3.UploadPartInput{
				Bucket:     aws.String(dstBucket),
				Key:        aws.String(key),
				PartNumber: aws.Int32(partNum),
				UploadId:   aws.String(uploadID),
				Body:       NewSectionReader(file, start, partLen),
			})
			if err != nil {
				errCh <- fmt.Errorf("part %d: failed to upload: %w", partNum, err)
				return
			}
			etags[idx] = s3types.CompletedPart{
				ETag:       upResp.ETag,
				PartNumber: aws.Int32(partNum),
			}
		}(i, partNum, start, partLen)
	}

	wg.Wait()
	close(errCh)
	for e := range errCh {
		if e != nil {
			// Abort the multipart upload on error
			s3client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(dstBucket),
				Key:      aws.String(key),
				UploadId: aws.String(uploadID),
			})
			return e
		}
	}

	_, err = s3client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(dstBucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &s3types.CompletedMultipartUpload{
			Parts: etags,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	return nil
}
