package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds counters for bytes and files downloaded.
var (
	TotalFiles int64 // Total number of files to download
	TotalBytes int64 // Total bytes to download

	ScannedFiles int64

	DownloadedFiles int64
	DownloadedBytes int64

	UploadedArchivedFiles int64
	UploadedFiles         int64
	UploadedBytes         int64
	metricsTicker         *time.Ticker

	statsLine  string
	statsMutex sync.Mutex
)

func StartMetrics(ctx context.Context) {
	// Start metrics reporter goroutine
	var (
		lastBytes, lastUpBytes int64
		lastTime               = time.Now()
		startTime              = time.Now()
	)

	metricsTicker = time.NewTicker(100 * time.Millisecond)
	go func() {
		//defer metricsTicker.Stop()
		log.Println("Starting metrics...")
		for {
			select {
			case <-ctx.Done():
				// Context is done, exit the goroutine
				return
			case <-metricsTicker.C:
				curBytes := atomic.LoadInt64(&DownloadedBytes)
				curUpBytes := atomic.LoadInt64(&UploadedBytes)
				now := time.Now()
				elapsed := now.Sub(lastTime)

				statsMutex.Lock()
				lastlen := len(statsLine)

				var remaining string
				if DownloadedBytes > 0 && TotalBytes > 0 && DownloadedBytes < TotalBytes {
					elapsedTime := now.Sub(startTime)
					rate := float64(DownloadedBytes) / elapsedTime.Seconds()
					if rate > 0 {
						remainingBytes := TotalBytes - DownloadedBytes
						remainingSeconds := float64(remainingBytes) / rate
						remainingDuration := time.Duration(remainingSeconds * float64(time.Second))
						remaining = fmt.Sprintf("ETA: ~%s", remainingDuration.Round(time.Minute))
					} else {
						remaining = "ETA: N/A"
					}
				} else {
					remaining = "ETA: N/A"
				}
				remaining = strings.TrimSuffix(remaining, "0s")

				statsLine = fmt.Sprintf("Download: %d/%d %s/%s (%s)  Scanned: %d  Upload: %d with %d %s (%s) %s",
					// #/#
					DownloadedFiles, TotalFiles,
					// #/#
					humanizeBytes(DownloadedBytes), humanizeBytes(TotalBytes),
					// ( )
					humanizeRate(curBytes-lastBytes, elapsed),
					// Scanned:
					ScannedFiles,
					// Upload:
					UploadedFiles,
					// with
					UploadedArchivedFiles, humanizeBytes(UploadedBytes),
					// ( )
					humanizeRate(curUpBytes-lastUpBytes, elapsed),
					//
					remaining)

				fmt.Fprintf(os.Stderr, "\r%s", statsLine)
				for i := len(statsLine); i < lastlen; i++ {
					fmt.Fprintf(os.Stderr, " ")
				}

				statsMutex.Unlock()

				lastBytes = curBytes
				lastUpBytes = curUpBytes
				lastTime = now
			}
		}
	}()
}

func Println(v ...any) {
	statsMutex.Lock()

	fmt.Fprintf(os.Stderr, "\r%s\r", spaces(len(statsLine)))
	fmt.Println(v...)

	statsMutex.Unlock()
}

func spaces(i int) (s string) {
	for i > 0 {
		s += " "
		i--
	}
	return s
}

func StopMetrics() {
	if metricsTicker != nil {
		log.Println("Metrics stopped...")
		metricsTicker.Stop()
	}
}

func humanizeBytes(bytes int64) string {
	const (
		_          = iota
		KB float64 = 1 << (10 * iota)
		MB
		GB
		TB
	)
	b := float64(bytes)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2f TiB", b/TB)
	case b >= GB:
		return fmt.Sprintf("%.2f GiB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.2f MiB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.2f KiB", b/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func humanizeRate(bytes int64, d time.Duration) string {
	if d <= 0 {
		return "N/A"
	}
	rate := float64(bytes) / d.Seconds()
	return fmt.Sprintf("%s/s", humanizeBytes(int64(rate)))
}
