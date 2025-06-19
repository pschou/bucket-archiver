package main

import (
	"context"
	"errors"
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
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
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

	tempName := outFile.Name()
	defer func() {
		outFile.Close()
		if tempName != "" {
			os.Remove(tempName)
		}
	}()

	if err := outFile.Truncate(size); err != nil {
		return "", fmt.Errorf("failed to pre-allocate file: %w", err)
	}

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
			return "", e
		}
	}

	tempName = "" // Prevent deletion
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
	defer file.Close()
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

	var partMiBs int64 = 10
	uploader := manager.NewUploader(s3client, func(u *manager.Uploader) {
		u.PartSize = partMiBs * 1024 * 1024
	})
	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(dstBucket),
		Key:    aws.String(key),
		Body:   &UploadReader{file},
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "EntityTooLarge" {
			log.Printf("Error while uploading object to %s. The object is too large.\n"+
				"The maximum size for a multipart upload is 5TB.", dstBucket)
		} else {
			log.Printf("Couldn't upload large object to %v:%v. Here's why: %v\n",
				dstBucket, key, err)
		}
	} else {
		err = s3.NewObjectExistsWaiter(s3client).Wait(
			ctx, &s3.HeadObjectInput{Bucket: aws.String(dstBucket), Key: aws.String(key)}, time.Minute)
		if err != nil {
			log.Printf("Failed attempt to wait for object %s to exist.\n", key)
		}
	}

	return err
}
