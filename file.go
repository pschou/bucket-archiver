package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
)

// FileStats holds the total file count and size.
type FileStats struct {
	Count int64 `json:"total_objects"`
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

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := fi.Size()
	if size == 0 {
		return nil, errors.New("file is empty")
	}

	f.Seek(-1000, io.SeekEnd)

	scanner := bufio.NewScanner(f)
	var lastLine string
	for scanner.Scan() {
		lastLine = scanner.Text()
		var stats FileStats
		if err := json.Unmarshal([]byte(lastLine), &stats); err == nil {
			return &stats, nil
		}
	}

	return nil, errors.New("no last line found")
}
