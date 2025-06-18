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
$ SRC_BUCKET=pj-src DST_BUCKET=pj-dst ./s3archiver
2025/06/18 16:43:33 EC2 Environment:
2025/06/18 16:43:33   AWS_REGION: us-east-1
2025/06/18 16:43:33   IMDS_ARN: arn:aws:iam::659486592847:instance-profile/ec2-s3-full-access
2025/06/18 16:43:33   IMDS_ID: AIPAZTDDWZ5H52NMAZHCM
Testing call to AWS...
  # The refresh interval for grabbing new AMI credentials
  REFRESH="20m" (default)
2025/06/18 16:43:33 Initializing ClamAV...
2025/06/18 16:43:52 db load succeed: 8706313
2025/06/18 16:43:57 engine compiled successfully
2025/06/18 16:43:57 ClamAV DB version: 27673
2025/06/18 16:43:57 ClamAV DB time: 2025-06-18 09:48:55 +0000 UTC
2025/06/18 16:43:57 Max scan size: 42949672960
2025/06/18 16:43:57 Max scan time: 90000
2025/06/18 16:43:57 Max file size: 2147483647
2025/06/18 16:43:57 ClamAV initialized successfully
2025/06/18 16:43:57 <virus_scan><vendor>ClamAV lib</vendor><version>27673</version><signature_date>2025-06-18T09:48:55Z</signature_date><result>pass</result></virus_scan>
2025/06/18 16:43:57 Source bucket: pj-src
2025/06/18 16:43:57 Destination bucket: pj-dst
2025/06/18 16:43:57 Size cap limit for each tarball contents: 1073741824 bytes
2025/06/18 16:43:57 metadata file metadata.jsonl already exists in the local filesystem
2025/06/18 16:43:57 Total objects: 42, Total size: 5284873635 bytes
2025/06/18 16:43:57 Starting to process metadata file: metadata.jsonl
1/42 0.00%: 10.txt
2/42 0.00%: 11.txt
3/42 0.00%: 12.txt
4/42 0.00%: 13.txt
5/42 0.00%: 14.txt
6/42 0.00%: 15.txt
7/42 0.00%: 16.txt
8/42 0.00%: 17.txt
9/42 0.00%: 18.txt
10/42 0.00%: 19.txt
11/42 0.00%: 20.txt
12/42 0.00%: test10.dat
13/42 3.23%: test11.dat
14/42 6.45%: test12.dat
15/42 9.68%: test13.dat
16/42 12.90%: test14.dat
17/42 16.13%: test15.dat
18/42 19.35%: test16.dat
Closing archive_0000000.tgz, compression: -0.03% (compressed: 1193677746 bytes, uncompressed: 1193358699 bytes)
Uploading archive_0000000.tgz to bucket pj-dst
19/42 22.58%: test17.dat
20/42 25.81%: test18.dat
2025/06/18 16:45:48 Uploaded archive_0000000.tgz to bucket pj-dst
21/42 29.03%: test19.dat
22/42 32.26%: test20.dat
23/42 35.48%: test21.dat
24/42 38.71%: test22.dat
25/42 41.94%: test23.dat
Closing archive_0000001.tgz, compression: -0.03% (compressed: 1193677480 bytes, uncompressed: 1193358523 bytes)
Uploading archive_0000001.tgz to bucket pj-dst
26/42 45.16%: test24.dat
2025/06/18 16:47:08 Uploaded archive_0000001.tgz to bucket pj-dst
27/42 48.39%: test25.dat
28/42 51.61%: test26.dat
29/42 54.84%: test27.dat
30/42 58.06%: test28.dat
...
```
