package main

import (
	"archive/zip"
	"compress/flate"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

func ZipFile(filePath string) (zipName string, err error) {
	log.Println("zipping file:", filePath)
	fi, err := os.Lstat(filePath)
	if err != nil {
		return "", fmt.Errorf("error while trying to access file: %s: %w", filePath, err)
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular file: %s", filePath)
	}

	rf, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error while opening file: %s: %w", filePath, err)
	}
	defer rf.Close()

	zipName = fmt.Sprintf("%s.zip", filePath)
	wf, err := os.Create(zipName)
	if err != nil {
		return "", fmt.Errorf("error while creating zip file: %s: %w", zipName, err)
	}
	defer wf.Close()

	zw := zip.NewWriter(wf)
	defer zw.Close()

	// Register a custom Deflate compressor.
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})

	zipFile, err := zw.Create(filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("error while creating zip file: %s: %w", filePath, err)
	}

	_, err = io.Copy(zipFile, rf)
	if err != nil {
		return "", fmt.Errorf("error while copying file: %s: %w", filePath, err)
	}
	return zipName, nil
}
