package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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

	uploadSWD = sizedwaitgroup.New(2) // Limit concurrent uploads to 2
)

func init() {
	/*sdkConfig, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal("Could not load default config,", err)
	}*/

	imdsClient := imds.New(imds.Options{})
	gro, err := imdsClient.GetRegion(context.TODO(), &imds.GetRegionInput{})
	if err != nil {
		log.Fatal("Could not get region property,", err)
	}

	iam, err := imdsClient.GetIAMInfo(context.TODO(), &imds.GetIAMInfoInput{})
	if err != nil {
		log.Fatal("Could not get IAM property,", err)
	}

	region = gro.Region
	log.Println("EC2 Environment:")
	log.Println("  AWS_REGION:", gro.Region)
	log.Println("  IMDS_ARN:", iam.IAMInfo.InstanceProfileArn)
	log.Println("  IMDS_ID:", iam.IAMInfo.InstanceProfileID)

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

	fmt.Println("Testing call to AWS...")
	if err := getConfig(); err != nil {
		log.Fatal("Error getting config:", err)
	}
	refreshTime, err := time.ParseDuration(Env("REFRESH", "20m", "The refresh interval for grabbing new AMI credentials"))

	go func() {
		// Refresh credentials every 20 minutes to ensure low latency on requests
		// and recovery should the server not have a policy assigned to it yet.
		for {
			time.Sleep(refreshTime)
			log.Printf("Pulling new creds for s3Client %#v\n", s3client)
			getConfig()
		}
	}()
}

func downloadObjectToTempFile(ctx context.Context, srcBucket string, key string) (string, error) {
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

	// Write the content of the S3 object to the temporary file
	if _, err := io.Copy(tmpFile, getObj.Body); err != nil {
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Ensure the temporary file is closed and return its name
	return tmpFile.Name(), nil
}

func downloadObjectToBuffer(ctx context.Context, srcBucket string, key string, buf []byte) (int, error) {
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
	uploadSWD.Add()
	go func(fileToUpload string) {
		defer uploadSWD.Done()
		if err := uploadFileToBucket(ctx, dstBucket, filePath, filePath); err != nil {
			log.Printf("Failed to upload %s: %v", filePath, err)
		} else {
			log.Printf("Uploaded %s to bucket %s", filePath, dstBucket)
		}
		os.Remove(fileToUpload) // Clean up the temporary file after upload
	}(filePath)
}

func uploadFileToBucket(ctx context.Context, dstBucket string, key string, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	_, err = s3client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(dstBucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to bucket %s with key %s: %w", dstBucket, key, err)
	}

	return nil
}
