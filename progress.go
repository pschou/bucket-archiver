package main

// Progress bar initialization
import (
	"bufio"
	"io"
	"sync/atomic"
)

func progressCp(rawdst io.Writer, src io.Reader, size int64) (int64, error) {
	// Progress copy function that reads from src and writes to dst while displaying progress
	dst := bufio.NewWriter(rawdst)
	defer dst.Flush() // Ensure all data is flushed to the writer

	buf := bufPool32.Get().([]byte)
	defer bufPool32.Put(buf)

	var written int64

	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				atomic.AddInt64(&written, int64(nw))
			}
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if er != nil {
			if er == io.EOF {
				break
			}
			return written, er
		}
	}
	return written, nil
}

/*
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
*/
