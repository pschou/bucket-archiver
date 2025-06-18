package main

import (
	"fmt"
	"log"

	clamav "github.com/hexahigh/go-clamav"
)

var (
	scanMutex      = make(chan struct{}, 1) // semaphore to limit concurrent scans
	clamavInstance *clamav.Clamav
)

func init() {
	log.Println("Initializing ClamAV...")
	// new clamav instance
	clamavInstance = new(clamav.Clamav)
	err := clamavInstance.Init(clamav.SCAN_OPTIONS{
		General:   0,
		Parse:     clamav.CL_SCAN_PARSE_ARCHIVE | clamav.CL_SCAN_PARSE_ELF,
		Heuristic: 0,
		Mail:      0,
		Dev:       0,
	})
	if err != nil {
		panic(err)
	}

	// free clamav memory
	//defer clamavInstance.Free()

	// load db (/var/lib/clamav/)
	signo, err := clamavInstance.LoadDB("./db", uint(clamav.CL_DB_DIRECTORY))
	if err != nil {
		panic(err)
	}
	log.Println("db load succeed:", signo)

	// compile engine
	err = clamavInstance.CompileEngine()
	if err != nil {
		panic(err)
	}
	log.Println("engine compiled successfully")

	// set max scansize
	// 40 GB
	// This is the maximum size of a file that can be scanned.
	// If a file exceeds this size, it will be skipped.
	// This is useful to prevent scanning large files that may take a long time to scan.
	// The value is in bytes, so 1024*1024*1024*40 = 40 GB.
	// Note: This is a very high value, and you may want to adjust it based on your use case.
	if err := clamavInstance.EngineSetNum(clamav.CL_ENGINE_MAX_SCANSIZE, 1024*1024*1024*40); err != nil {
		log.Fatalln("Could not set max scan size", err)
	}
	maxScanSize, err := clamavInstance.EngineGetNum(clamav.CL_ENGINE_MAX_SCANSIZE)
	if err != nil {
		log.Fatalln("Could not get max scan size", err)
	}
	log.Println("Max scan size:", maxScanSize)

	// set max scan time
	// 90000 milliseconds = 90 seconds
	// This is the maximum time allowed for a scan before it is aborted.
	// This is useful to prevent long-running scans from hanging indefinitely.
	if err = clamavInstance.EngineSetNum(clamav.CL_ENGINE_MAX_SCANTIME, 90000); err != nil {
		log.Fatalln("Could not set max scan time", err)
	}
	maxScanTime, err := clamavInstance.EngineGetNum(clamav.CL_ENGINE_MAX_SCANTIME)
	if err != nil {
		log.Fatalln("Could not get max scan time", err)
	}
	log.Println("Max scan time:", maxScanTime)

	// set max file size
	// 2 GB
	// This is the maximum size of a file that can be scanned.
	// If a file exceeds this size, it will be skipped.
	// This is useful to prevent scanning large files that may take a long time to scan.
	// The value is in bytes, so 2*1024*1024*1024 = 2 GB.
	if err = clamavInstance.EngineSetNum(clamav.CL_ENGINE_MAX_FILESIZE, 2*1024*1024*1024-1); err != nil {
		log.Fatalln("Could not set max file size", err)
	}
	maxFileSize, err := clamavInstance.EngineGetNum(clamav.CL_ENGINE_MAX_FILESIZE)
	if err != nil {
		log.Fatalln("Could not get max file size", err)
	}
	log.Println("Max file size:", maxFileSize)

	log.Println("ClamAV initialized successfully")
}

// ScanFileSerial scans a single file, ensuring only one scan runs at any given instance.
func ScanFile(filePath string) (scanned uint, err error) {
	scanMutex <- struct{}{}        // acquire semaphore (only 1 allowed)
	defer func() { <-scanMutex }() // release semaphore

	// scan
	var virusName string
	scanned, virusName, err = clamavInstance.ScanFile(filePath)
	if virusName != "" {
		//log.Printf("Virus found in %q: %s\n", filePath, virusName)
		// If a virus is found, return an error with the virus name
		// and the file path for clarity.}
		return scanned, fmt.Errorf("In %q found %q", filePath, virusName)
	} else if err != nil {
		//log.Println("Error scanning file:", err)
		return scanned, fmt.Errorf("error scanning file %q: %w", filePath, err)
	}

	return scanned, nil
}
