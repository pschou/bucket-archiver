package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"time"

	clamav "github.com/hexahigh/go-clamav"
)

var (
	scanMutex      = make(chan struct{}, 1) // semaphore to limit concurrent scans
	clamavInstance *clamav.Clamav

	virusScanMeta    = Virus_scan{Vendor: "ClamAV lib", Result: "pass"}
	virusScanMetaXML []byte                // XML representation of the virus scan metadata
	virusScanMap     = map[string]string{} // Metadata map for virus scan
)

type Virus_scan struct {
	XMLName        xml.Name `xml:"virus_scan" json:"virus_scan"`
	Text           string   `xml:",chardata"`
	Vendor         string   `xml:"vendor" json:"vendor"`
	Version        string   `xml:"version" json:"version"`
	Signature_date string   `xml:"signature_date" json:"signature_date"`
	Result         string   `xml:"result" json:"result"`
}

func init() {
	log.Println("Initializing ClamAV...")
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

	// get db version
	// This is the version of the ClamAV database.
	// It is useful to know the version of the database to ensure it is up-to-date.
	// The version is a number that represents the version of the database.
	dbVersion, err := clamavInstance.EngineGetNum(clamav.CL_ENGINE_DB_VERSION)
	if err != nil {
		log.Fatalln("Could not get ClamAV DB version", err)
	}
	log.Println("ClamAV DB version:", dbVersion)
	virusScanMeta.Version = fmt.Sprintf("%d", dbVersion)

	// get db time
	// This is the time when the database was last updated.
	// It is useful to know when the database was last updated to ensure it is up-to-date.
	dbTime, err := clamavInstance.EngineGetNum(clamav.CL_ENGINE_DB_TIME)
	if err != nil {
		log.Fatalln("Could not get ClamAV DB time", err)
	}
	log.Println("ClamAV DB time:", time.Unix(int64(dbTime), 0))
	virusScanMeta.Signature_date = time.Unix(int64(dbTime), 0).Format(time.RFC3339)

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
	virusScanMetaXML, _ = xml.Marshal(virusScanMeta)

	log.Println("ClamAV initialized successfully")
	log.Println(string(virusScanMetaXML))
	virusScanMap["scan"] = string(virusScanMetaXML)
}
