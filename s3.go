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
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	size := info.Size()
	if partCount <= 0 || size == 0 {
		return fmt.Errorf("invalid partCount or file size")
	}

	s3Ready.Wait() // Wait for the S3 client to be ready
	//func multipartUpload(client *s3.Client, body []byte, uploadPath string, contType string, fileSize int64, bucket string) error {
	input := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(dstBucket),
		Key:         aws.String(filePath),
		ContentType: aws.String("application/gzip"),
	}
	resp, err := s3client.CreateMultipartUpload(context.TODO(), input)
	if err != nil {
		return err
	}

	var curr, partLength int64
	var remaining = size
	var completedParts []types.CompletedPart
	const maxPartSize int64 = int64(50 * 1024 * 1024)
	partNumber := int32(1)
	for curr = 0; remaining != 0; curr += partLength {
		if remaining < maxPartSize {
			partLength = remaining
		} else {
			partLength = maxPartSize
		}

		partInput := &s3.UploadPartInput{
			Body:       NewSectionReader(file, curr, partLength),
			Bucket:     resp.Bucket,
			Key:        resp.Key,
			PartNumber: aws.Int32(partNumber),
			UploadId:   resp.UploadId,
		}
		uploadResult, err := s3client.UploadPart(context.TODO(), partInput)
		if err != nil {
			aboInput := &s3.AbortMultipartUploadInput{
				Bucket:   resp.Bucket,
				Key:      resp.Key,
				UploadId: resp.UploadId,
			}
			_, aboErr := s3client.AbortMultipartUpload(context.TODO(), aboInput)
			if aboErr != nil {
				return aboErr
			}
			return err
		}

		completedParts = append(completedParts, types.CompletedPart{
			ETag:       uploadResult.ETag,
			PartNumber: aws.Int32(partNumber),
		})
		remaining -= partLength
		partNumber++
	}

	compInput := &s3.CompleteMultipartUploadInput{
		Bucket:   resp.Bucket,
		Key:      resp.Key,
		UploadId: resp.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}
	_, compErr := s3client.CompleteMultipartUpload(context.TODO(), compInput)
	if err != nil {
		return compErr
	}

	return nil
}
