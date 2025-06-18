package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
)

// FileStats holds the total file count and size.
type FileStats struct {
	Count int   `json:"total_objects"`
	Size  int64 `json:"total_size"`
}

// ReadLastLineJSONStats seeks to the end of the file, reads the last line,
// and parses it as JSON to extract file count and size.
func ReadLastLineJSONStats(path string) (*FileStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	const readBlockSize = 4096
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	if size == 0 {
		return nil, errors.New("file is empty")
	}

	var (
		buf      []byte
		offset   int64
		readSize int
	)
	offset = size
	for {
		if offset < readBlockSize {
			readSize = int(offset)
			offset = 0
		} else {
			readSize = readBlockSize
			offset -= int64(readSize)
		}
		tmp := make([]byte, readSize)
		_, err := f.ReadAt(tmp, offset)
		if err != nil && err != io.EOF {
			return nil, err
		}
		buf = append(tmp, buf...)
		if idx := strings.LastIndex(string(buf), "\n"); idx != -1 && offset != 0 {
			buf = buf[idx+1:]
			break
		}
		if offset == 0 {
			break
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(string(buf)))
	var lastLine string
	for scanner.Scan() {
		lastLine = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if lastLine == "" {
		return nil, errors.New("no last line found")
	}

	var stats FileStats
	if err := json.Unmarshal([]byte(lastLine), &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}
