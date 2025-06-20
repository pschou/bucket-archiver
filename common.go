package main

import (
	"fmt"
	"os"
	"sync"
)

var (
	// bufPool is a sync.Pool to reuse byte slices for copying data
	bufPool32 = sync.Pool{
		New: func() interface{} {
			return make([]byte, 32*1024)
		},
	} // bufPool is a sync.Pool to reuse byte slices for copying data
	bufPoolLarge = sync.Pool{
		New: func() interface{} {
			return make([]byte, maxMemObject*1024)
		},
	}
)

func Env(env, def, usage string) string {
	if e := os.Getenv(env); len(e) > 0 {
		fmt.Printf("  %-30s # %s\n", fmt.Sprintf("%s=%q", env, e), usage)
		return e
	}
	fmt.Printf("  %-30s # %s\n", fmt.Sprintf("%s=%q (default)", env, def), usage)
	return def
}

func EnvInt(env string, def int, usage string) int {
	valStr := os.Getenv(env)
	if valStr != "" {
		var val int
		_, err := fmt.Sscanf(valStr, "%d", &val)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid integer for %s: %q\n", env, valStr)
			os.Exit(1)
		}
		fmt.Printf("  %-30s # %s\n", fmt.Sprintf("%s=%d", env, val), usage)
		return val
	}
	fmt.Printf("  %-30s # %s\n", fmt.Sprintf("%s=%d (default)", env, def), usage)
	return def
}

// parseByteSize parses a human-readable byte size string (e.g., "1GB", "500MB", "100K") into int64 bytes.
func parseByteSize(s string) (int64, error) {
	var size int64
	var unit string
	n, err := fmt.Sscanf(s, "%d%s", &size, &unit)
	if n < 1 || err != nil {
		return 0, fmt.Errorf("invalid size format: %q", s)
	}
	switch unit {
	case "", "B", "b":
		return size, nil
	case "K", "KB", "k", "kb":
		return size * 1024, nil
	case "M", "MB", "m", "mb":
		return size * 1024 * 1024, nil
	case "G", "GB", "g", "gb":
		return size * 1024 * 1024 * 1024, nil
	case "T", "TB", "t", "tb":
		return size * 1024 * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unknown size unit: %q", unit)
	}
}
