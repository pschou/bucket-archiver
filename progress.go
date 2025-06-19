package main

// Progress bar initialization
import (
	progressBar "github.com/elulcao/progress-bar/cmd" // Importing the progress bar package
)

var ( // progress is the global progress bar instance
	// It is initialized with the NewPBar function from the progressCmd package.
	// This progress bar will be used throughout the application to show progress updates.
	progress = progressBar.NewPBar()
)

func init() {
	progress.SignalHandler()
}
