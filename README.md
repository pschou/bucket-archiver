# Amazon S3 Bucket Archiving Tool

## Overview

The Amazon S3 Bucket Archiving Tool is designed to efficiently archive a bucket containing numerous small files into a single tarball. This tool simplifies the management of files stored in S3 by consolidating them, reducing storage complexity and enhancing data transfer efficiency. Additionally, it incorporates ClamAV scanning to ensure that all files are free from malware before being archived.

## Features

- **Archive Small Files**: Combines multiple small files into a single tarball for efficient storage.
- **Cloud Storage**: Supports interaction with Amazon S3 for both input and output operations.
- **ClamAV Integration**: Scans each file in transit using ClamAV to ensure files are safe and free from malware.
- **Easy Configuration**: User-friendly setup process to specify source and destination buckets.
- **Logging**: Maintains a log of processed files and any detected malware for traceability.

## Prerequisites

Before using the archiving tool, ensure you have:

- An AWS account with access to S3.
- AWS CLI configured with appropriate permissions for accessing both the source and destination S3 buckets.
- ClamAV installed on your system.

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/pschou/s3-archiving-tool.git
   cd s3-archiving-tool
   ```

2. Install any required dependencies:
   ```bash
   yum install clamav
   ```

3. Ensure you have the necessary IAM permissions to access S3 and run ClamAV.

## Usage

To use the archiving tool, follow these steps:

1. Modify the environment variables to specify:
   - `SRC_BUCKET`: The name of the S3 bucket containing the files to archive.
   - `DST_BUCKET`: The name of the S3 bucket where the archived tarball will be uploaded.
   - `SIZECAP`   : Size cap for all the files included into the archive

2. Run the archiving script:
   ```bash
   SRC_BUCKET=my_src DST_BUCKET=my_dst s3archiver
   ```

3. Monitor the logs to ensure all files are processed successfully and check for any malware alerts.

Files will be created with the names like archive_0000001.tgz and counting up.

## ClamAV Scanning

The tool will invoke ClamAV for each file being archived. Ensure that ClamAV is up to date to provide the best possible malware detection. If any files are found to be infected, they will be logged, and the archiving process will stop for those specific files, allowing for further investigation.

## Logging

Logs will be generated in the `logs` directory. The log files will contain details including:

- Timestamps of processing
- Files scanned
- ClamAV scan results
- Any errors encountered during the process

## Troubleshooting

- Ensure you have the appropriate permissions set in AWS IAM for accessing S3 buckets.
- If ClamAV scanning fails, verify that ClamAV is properly installed and accessible from the command line.
- Review log files for detailed error messages.

## Conclusion

The Amazon S3 Bucket Archiving Tool provides a simple yet effective way to manage large numbers of small files in S3. With integrated ClamAV scanning, you can maintain a higher level of security for your archived data. For further inquiries or support, please reach out via the issues section of the repository.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## Example Usage

First make sure clamav-lib is installed.  If there is an error with the installed version of clamav and the compiled binary, use the build.sh to build a new binary.

```
$ SRC_BUCKET=pj-src DST_BUCKET=pj-dst CONCURRENT_SCANNERS=16 MAX_IN_MEM=1024 CHAN_DOWNLOADED_FILES=200 PREFIX_FILTER='userdata/' ARCHIVE_NAME="prescan/archive_bigboy_%07d.tgz" SIZECAP="8G" ./s3archiver
  MAX_IN_MEM=1024                # Maximum in memory object in kb
  ARCHIVE_NAME="prescan/archive_bigboy_%07d.tgz" # Output template
  CONCURRENT_SCANNERS=16         # How many concurrent scanners can run at once
awscli: 2025/06/20 15:51:49 Initializing S3 client...
  REFRESH="20m" (default)        # The refresh interval for grabbing new AMI credentials
  SRC_BUCKET="pj-src"            # The source S3 bucket name
  DST_BUCKET="pj-dst"            # The destination S3 bucket name
clamav: 2025/06/20 15:51:49 Initializing ClamAV...
  DEFINITIONS="./db" (default)   # The path with the ClamAV definitions
  MAX_SCANTIME=180000 (default)  # Max scan time in milliseconds
Starting bucket-archiver v20250620.1550: downloading, archiving, and uploading S3 objects.
  SIZECAP="8G"                   # Limit the size of the uncompressed archive payload
2025/06/20 15:51:49 Making pipeline channels.
  CHAN_TODO_DOWNLOAD=10 (default) # Buffer size for toDownload channel
  CHAN_DOWNLOADED_FILES=200      # Buffer size for downloadedFiles channel
  CHAN_SCANNED_FILES=10 (default) # Buffer size for scannedFiles channel
  CHAN_ARCHIVE_FILES=2 (default) # Buffer size for ArchiveFiles channel
2025/06/20 15:51:49 metadata file metadata.jsonl already exists in the local filesystem
2025/06/20 15:51:49 Total objects: 8800150, Total size: 2.57 TiB
awscli: 2025/06/20 15:51:49 EC2 Environment:
awscli: 2025/06/20 15:51:49   AWS_REGION: us-east-1
awscli: 2025/06/20 15:51:49   IMDS_ARN: arn:aws:iam::751442555555:instance-profile/EC2SSMRole
awscli: 2025/06/20 15:51:49   IMDS_ID: AIPA255LXON7UAKM4LAGS
awscli: 2025/06/20 15:51:49 Testing call to AWS...
awscli: 2025/06/20 15:51:49 S3 client initialized successfully
clamav: 2025/06/20 15:52:03 db load succeed: 8706316
clamav: 2025/06/20 15:52:08 engine compiled successfully
clamav: 2025/06/20 15:52:08 ClamAV DB version: 27675
clamav: 2025/06/20 15:52:08 ClamAV DB time: 2025-06-20 08:35:04 +0000 UTC
clamav: 2025/06/20 15:52:08 Max scan size: 42949672960
clamav: 2025/06/20 15:52:08 Max scan time: 180000
clamav: 2025/06/20 15:52:08 Max file size: 2147483647
clamav: 2025/06/20 15:52:08 ClamAV initialized successfully
2025/06/20 15:52:08 Watching for errors...
2025/06/20 15:52:08 Starting uploader...
2025/06/20 15:52:08 Starting downloader...
2025/06/20 15:52:08 Starting scanner...
2025/06/20 15:52:08 Reading in metadata.jsonl for processing...
2025/06/20 15:52:08 Starting metrics...
2025/06/20 15:52:08 Starting archiver...
Download: 7514/8800150 3.22 GiB/2.57 TiB (0 B/s)  Scanned: 7297  Upload: 0 0 B (0 B/s) ETA: ~60h49m0s
...
```
