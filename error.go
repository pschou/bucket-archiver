package main

var (
	fileErrCh = make(chan *ErrorEvent, 100) // Channel to send error events
)

type ErrorEvent struct {
	Filename string // Name of the file that caused the error
	Size     int64  // Size of the file that caused the error
	Read     int64  // Number of bytes read before the error occurred
	Err      error  // The error that occurred
}
