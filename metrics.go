package main

import (
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"
)

// Metrics holds counters for bytes and files downloaded.
var (
	TotalFiles int64 // Total number of files to download
	TotalBytes int64 // Total bytes to download

	DownloadedFiles int64
	DownloadedBytes int64
	metricsTicker   *time.Ticker
)

func StartMetrics() {
	// Start metrics reporter goroutine
	var lastBytes int64
	var lastTime = time.Now()

	metricsTicker = time.NewTicker(250 * time.Millisecond)
	go func() {
		//defer metricsTicker.Stop()
		log.Println("Starting metrics...")
		for range metricsTicker.C {
			curBytes := atomic.LoadInt64(&DownloadedBytes)
			now := time.Now()
			elapsed := now.Sub(lastTime)

			fmt.Fprintf(os.Stderr, "\rDownload: %d/%d %s/%s (%s)", DownloadedFiles, TotalFiles,
				humanizeBytes(DownloadedBytes), humanizeBytes(TotalBytes), humanizeRate(curBytes-lastBytes, elapsed))
			lastBytes = curBytes
			lastTime = now
		}
	}()
}

func StopMetrics() {
	if metricsTicker != nil {
		metricsTicker.Stop()
		log.Println("Metrics stopped...")
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
