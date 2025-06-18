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

1. Modify the configuration file (`config.json`) to specify:
   - `source_bucket`: The name of the S3 bucket containing the files to archive.
   - `destination_bucket`: The name of the S3 bucket where the archived tarball will be uploaded.
   - `clamav_path`: The path to the ClamAV executable.

   ```json
   {
     "source_bucket": "your-source-bucket",
     "destination_bucket": "your-destination-bucket",
     "clamav_path": "/usr/local/bin/clamscan"
   }
   ```

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

Feel free to customize any section to better fit your project's needs!
