package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	clamav "github.com/hexahigh/go-clamav"
	"github.com/remeh/sizedwaitgroup"
)

var (
	clamavInstance *clamav.Clamav        // ClamAV instance for scanning files
	virusScanMap   = map[string]string{} // Metadata map for virus scan
	scanReady      sync.WaitGroup        // channel to signal scan readiness

	clamLog         = log.New(os.Stderr, "clamav: ", log.LstdFlags)
	concurrentScans = EnvInt("CONCURRENT_SCANNERS", 3, "How many concurrent scanners can run at once")
)

// ScannedFile represents a file that has been scanned.
type ScannedFile struct {
	Size     int64
	Filename string

	TempFile string // Temporary file path if the file is large.
	Bytes    []byte // If the file is small, we can keep it in memory.
}

func init() {
	clamLog.Println("Initializing ClamAV...")
	definitionsPath := Env("DEFINITIONS", "./db", "The path with the ClamAV definitions")
	// Test if path exists and can be read or fail
	info, err := os.Stat(definitionsPath)
	if err != nil {
		clamLog.Fatalf("Definitions path error: %v", err)
	}
	if !info.IsDir() {
		clamLog.Fatalf("Definitions path is not a directory: %s", definitionsPath)
	}
	file, err := os.Open(definitionsPath)
	if err != nil {
		clamLog.Fatalf("Cannot read definitions path: %v", err)
	}
	file.Close()

	scanReady.Add(1) // Add to wait group to signal when ClamAV is ready
	go func() {
		defer scanReady.Done() // Signal that the ClamAV instance is ready

		// new clamav instance
		clamavInstance = new(clamav.Clamav)
		err := clamavInstance.Init(clamav.SCAN_OPTIONS{
			General:   clamav.CL_SCAN_GENERAL_ALLMATCHES,
			Parse:     ^uint(0), // clamav.CL_SCAN_PARSE_ARCHIVE | clamav.CL_SCAN_PARSE_ELF,
			Heuristic: clamav.CL_SCAN_HEURISTIC_EXCEEDS_MAX,
			Mail:      0,
			Dev:       0,
		})
		if err != nil {
			panic(err)
		}

		// free clamav memory
		//defer clamavInstance.Free()

		// load db (/var/lib/clamav/)
		signo, err := clamavInstance.LoadDB(definitionsPath, uint(clamav.CL_DB_DIRECTORY))
		if err != nil {
			panic(err)
		}
		clamLog.Println("db load succeed:", signo)

		// compile engine
		err = clamavInstance.CompileEngine()
		if err != nil {
			panic(err)
		}
		clamLog.Println("engine compiled successfully")
		virusScanMap["vendor"] = "ClamAV lib"

		// get db version
		// This is the version of the ClamAV database.
		// It is useful to know the version of the database to ensure it is up-to-date.
		// The version is a number that represents the version of the database.
		dbVersion, err := clamavInstance.EngineGetNum(clamav.CL_ENGINE_DB_VERSION)
		if err != nil {
			clamLog.Fatalln("Could not get ClamAV DB version", err)
		}
		clamLog.Println("ClamAV DB version:", dbVersion)
		virusScanMap["version"] = fmt.Sprintf("%d", dbVersion)

		// get db time
		// This is the time when the database was last updated.
		// It is useful to know when the database was last updated to ensure it is up-to-date.
		dbTime, err := clamavInstance.EngineGetNum(clamav.CL_ENGINE_DB_TIME)
		if err != nil {
			clamLog.Fatalln("Could not get ClamAV DB time", err)
		}
		clamLog.Println("ClamAV DB time:", time.Unix(int64(dbTime), 0))
		virusScanMap["signature_date"] = time.Unix(int64(dbTime), 0).Format(time.RFC3339)

		// set max scansize
		// 40 GB
		// This is the maximum size of a file that can be scanned.
		// If a file exceeds this size, it will be skipped.
		// This is useful to prevent scanning large files that may take a long time to scan.
		// The value is in bytes, so 1024*1024*1024*40 = 40 GB.
		// Note: This is a very high value, and you may want to adjust it based on your use case.
		if err := clamavInstance.EngineSetNum(clamav.CL_ENGINE_MAX_SCANSIZE, 1024*1024*1024*40); err != nil {
			clamLog.Fatalln("Could not set max scan size", err)
		}
		maxScanSize, err := clamavInstance.EngineGetNum(clamav.CL_ENGINE_MAX_SCANSIZE)
		if err != nil {
			clamLog.Fatalln("Could not get max scan size", err)
		}
		clamLog.Println("Max scan size:", maxScanSize)

		// set max scan time
		// 90000 milliseconds = 90 seconds
		// This is the maximum time allowed for a scan before it is aborted.
		// This is useful to prevent long-running scans from hanging indefinitely.
		if err = clamavInstance.EngineSetNum(clamav.CL_ENGINE_MAX_SCANTIME, 90000); err != nil {
			clamLog.Fatalln("Could not set max scan time", err)
		}
		maxScanTime, err := clamavInstance.EngineGetNum(clamav.CL_ENGINE_MAX_SCANTIME)
		if err != nil {
			clamLog.Fatalln("Could not get max scan time", err)
		}
		clamLog.Println("Max scan time:", maxScanTime)

		// set max file size
		// 2 GB
		// This is the maximum size of a file that can be scanned.
		// If a file exceeds this size, it will be skipped.
		// This is useful to prevent scanning large files that may take a long time to scan.
		// The value is in bytes, so 2*1024*1024*1024 = 2 GB.
		if err = clamavInstance.EngineSetNum(clamav.CL_ENGINE_MAX_FILESIZE, 2*1024*1024*1024-1); err != nil {
			clamLog.Fatalln("Could not set max file size", err)
		}
		maxFileSize, err := clamavInstance.EngineGetNum(clamav.CL_ENGINE_MAX_FILESIZE)
		if err != nil {
			clamLog.Fatalln("Could not get max file size", err)
		}
		clamLog.Println("Max file size:", maxFileSize)

		clamLog.Println("ClamAV initialized successfully")

		virusScanMap["result"] = "pass"
	}()
}

// Scanner listens for Downloaded on tasksCh, scans them, and sends ScannedFile to doneCh.
func Scanner(ctx context.Context, tasksCh <-chan DownloadedFile, doneCh chan<- ScannedFile) {
	log.Println("Starting scanner...")
	swg := sizedwaitgroup.New(concurrentScans)
	defer close(doneCh) // Ensure doneCh is closed when the function exits

	scanReady.Wait() // Wait for the ClamAV instance to be ready

	for {
		select {
		case <-ctx.Done():
			break
		case task, ok := <-tasksCh:
			if !ok {
				swg.Wait()
				Println("Closing scanner...")
				return
			}

			swg.Add()
			go func(task DownloadedFile) {
				defer swg.Done()
				defer atomic.AddInt64(&ScannedFiles, 1)

				if task.Size == 0 {
					doneCh <- ScannedFile{
						Size:     task.Size,
						Filename: task.Filename,
					}

					return // Skip empty files
				}

				if task.TempFile == "" {
					// If the file is small enough, we can scan it in memory
					fmem := clamav.OpenMemory(task.Bytes)
					if fmem == nil {
						errCh <- &ErrorEvent{
							Size:     task.Size,
							Filename: task.Filename,
							Err:      fmt.Errorf("failed to open memory for scanning %s", task.Filename),
						}
						putMemory(task.Bytes)
						return // Skip this file if memory scan fails
					}
					// Scan the file in memory
					_, virusName, err := clamavInstance.ScanMapCB(fmem, task.Filename, context.Background())
					//clamav.CloseMemory(fmem) // Clean up memory after scanning

					if virusName != "" {
						//log.Printf("Virus found in %q: %s\n", filePath, virusName)
						// If a virus is found, return an error with the virus name
						// and the file path for clarity.}
						errCh <- &ErrorEvent{
							Size:     task.Size,
							Filename: task.Filename,
							Err:      fmt.Errorf("virus found in %s: %s", task.Filename, virusName),
						}
						putMemory(task.Bytes)
						return // Skip this file if memory scan fails
					} else if err != nil {
						errCh <- &ErrorEvent{
							Size:     task.Size,
							Filename: task.Filename,
							Err:      fmt.Errorf("error scanning %s: %v", task.Filename, err),
						}
						putMemory(task.Bytes)
						return // Skip this file if memory scan fails
					}
					doneCh <- ScannedFile{
						Size:     task.Size,
						Filename: task.Filename,
						TempFile: task.TempFile,
						Bytes:    task.Bytes,
					}
				} else {
					// If the file is large, we scan it from a temporary file
					// Scan the file
					//fmt.Printf("Scanning file: %s\n", tempFilePath)
					_, virusName, err := clamavInstance.ScanFile(task.TempFile)
					if virusName != "" {
						// If a virus is found, return an error with the virus name
						// and the file path for clarity.}
						errCh <- &ErrorEvent{
							Size:     task.Size,
							Filename: task.Filename,
							Err:      fmt.Errorf("virus found in %s: %s", task.Filename, virusName),
						}
						os.Remove(task.TempFile) // Clean up the temporary file after scanning
						return                   // Skip this file if a virus is found
					} else if err != nil {
						// If a virus is found, return an error with the virus name
						// and the file path for clarity.}
						errCh <- &ErrorEvent{
							Size:     task.Size,
							Filename: task.Filename,
							Err:      fmt.Errorf("error scanning %s: %v", task.Filename, err),
						}
						os.Remove(task.TempFile) // Clean up the temporary file after scanning
						return                   // Skip this file if a virus is found
					}
					doneCh <- ScannedFile{
						Size:     task.Size,
						Filename: task.Filename,
						TempFile: task.TempFile,
					}
				}
			}(task)
		}
	}
}
