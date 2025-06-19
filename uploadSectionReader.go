package main

import (
	"io"
	"sync/atomic"
)

type DownloadReader struct {
	r io.Reader
}

func (s *DownloadReader) Read(p []byte) (n int, err error) {
	n, err = s.r.Read(p)
	DownloadedBytes += int64(n)
	atomic.AddInt64(&UploadedBytes, int64(n))
	return
}

// NewSectionReader returns a [SectionReader] that reads from r
// starting at offset off and stops with EOF after n bytes.
func NewSectionReader(r io.ReaderAt, off int64, n int64) *SectionReader {
	var remaining int64
	const maxint64 = 1<<63 - 1
	if off <= maxint64-n {
		remaining = n + off
	} else {
		// Overflow, with no way to return error.
		// Assume we can read up to an offset of 1<<63 - 1.
		remaining = maxint64
	}
	return &SectionReader{r, off, off, remaining, n}
}

// SectionReader implements Read, Seek, and ReadAt on a section
// of an underlying [ReaderAt].
type SectionReader struct {
	r     io.ReaderAt // constant after creation
	base  int64       // constant after creation
	off   int64
	limit int64 // constant after creation
	n     int64 // constant after creation
}

func (s *SectionReader) Read(p []byte) (n int, err error) {
	if s.off >= s.limit {
		return 0, io.EOF
	}
	if max := s.limit - s.off; int64(len(p)) > max {
		p = p[0:max]
	}
	n, err = s.r.ReadAt(p, s.off)
	s.off += int64(n)
	atomic.AddInt64(&UploadedBytes, int64(n))
	return
}
