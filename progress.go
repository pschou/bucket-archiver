package main

// Progress bar initialization
import (
	"bufio"
	"fmt"
	"io"
	"sync"
	"time"
)

var (
	// bufPool is a sync.Pool to reuse byte slices for copying data
	bufPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 32*1024)
		},
	}
)

func progressCp(rawdst io.Writer, src io.Reader, size int64, file string, remainObj int, remainBytes int64) (int64, error) {
	// Progress copy function that reads from src and writes to dst while displaying progress
	dst := bufio.NewWriter(rawdst)
	defer dst.Flush() // Ensure all data is flushed to the writer

	// Truncate file name if it exceeds 60 characters
	file = truncateFileName(file, 60)
	buf := bufPool.Get().([]byte)
	defer bufPool.Put(buf)

	var (
		written   int64
		lastPrint = time.Now()
		startTime = time.Now()
	)

	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if time.Since(lastPrint) > 250*time.Millisecond {
			elapsed := time.Since(startTime)
			rate := humanizeRate(written, elapsed)
			fmt.Printf("\r%s: %s/%s (%s) with %d (%s) remaining", file,
				humanizeBytes(written), humanizeBytes(size), rate, remainObj, humanizeBytes(remainBytes))
			lastPrint = time.Now()
		}
		if er != nil {
			if er == io.EOF {
				break
			}
			return written, er
		}
	}
	elapsed := time.Since(startTime)
	rate := humanizeRate(written, elapsed)
	fmt.Printf("\r%s: %s/%s (%s) with %d (%s) remaining\n", file,
		humanizeBytes(written), humanizeBytes(size), rate, remainObj, humanizeBytes(remainBytes))
	return written, nil
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

func truncateFileName(file string, length int) string {
	// Truncate file name if longer than `length` characters, preserving extension in suffix
	// Also truncate any path down to 15 characters

	slash := -1
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '/' || file[i] == '\\' {
			slash = i
			break
		}
	}
	if slash != -1 {
		path := file[:slash]
		name := file[slash+1:]
		if len(path) > 15 {
			path = path[:10] + "..." + path[len(path)-2:]
		}
		file = path + "/" + name
	}
	if len(file) > length {
		ext := ""
		dot := -1
		for i := len(file) - 1; i >= 0; i-- {
			if file[i] == '.' {
				dot = i
				break
			}
			if file[i] == '/' || file[i] == '\\' {
				break
			}
		}
		if dot != -1 {
			ext = file[dot:]
		}
		// Reserve 3 for "..."
		remain := length - 3
		prefixLen := remain / 2
		suffixLen := remain - prefixLen
		if len(ext) > 0 && suffixLen < len(ext) {
			suffixLen = len(ext)
			prefixLen = remain - suffixLen
			if prefixLen < 0 {
				prefixLen = 0
			}
		}
		suffixStart := len(file) - suffixLen
		if len(ext) > 0 && suffixStart > dot {
			suffixStart = dot
			suffixLen = len(file) - dot
			prefixLen = remain - suffixLen
			if prefixLen < 0 {
				prefixLen = 0
			}
		}
		file = file[:prefixLen] + "..." + file[suffixStart:]
	}
	return file
}
