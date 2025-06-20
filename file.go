package main

import (
	"bufio"
	"bytes"
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

	const readBlockSize = 4096
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	if size == 0 {
		return nil, errors.New("file is empty")
	}

	offset := size
	if offset > 1000 {
		offset -= 1000
	}
	tmp := make([]byte, 1000)
	_, err = f.ReadAt(tmp, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(tmp))
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
