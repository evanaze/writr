package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed web/*
var webFS embed.FS

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "writr: error getting working directory: %v\n", err)
		os.Exit(1)
	}

	db, err := openDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "writr: error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	var openFile string
	sidebarOpen := true

	if len(os.Args) > 1 {
		filename := os.Args[1]
		// If the file exists, open it directly
		if _, err := os.Stat(filename); err == nil {
			openFile = filename
		} else if _, err := os.Stat(filepath.Join(root, filename)); err == nil {
			openFile = filename
		} else {
			// Create the file if it doesn't exist
			fullPath := filepath.Join(root, filename)
			if !filepath.IsAbs(filename) {
				fullPath = filepath.Join(root, filename)
			}
			dir := filepath.Dir(fullPath)
			os.MkdirAll(dir, 0755)
			os.WriteFile(fullPath, []byte(""), 0644)
			openFile = filename
		}
		sidebarOpen = false
	}

	if err := startServer(root, db, openFile, sidebarOpen); err != nil {
		fmt.Fprintf(os.Stderr, "writr: server error: %v\n", err)
		os.Exit(1)
	}
}