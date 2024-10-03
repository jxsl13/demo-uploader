package main

import (
	"context"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/jxsl13/demo-uploader/internal/testutils"
)

var (
	webhookUrl = "https://discord.com/api/webhooks/12345...."
)

func TestWatch(t *testing.T) {
	watchPath := testutils.FilePath("tmp")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	re := regexp.MustCompile(".*")

	err := Watch(ctx, webhookUrl, watchPath, re, time.Second*10)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpload(t *testing.T) {
	filePath := testutils.FilePath("tmp/test.txt")
	err := Upload(webhookUrl, filePath)
	if err != nil {
		t.Fatal(err)
	}
}

func TestZip(t *testing.T) {
	filePath := testutils.FilePath("tmp/test.txt")
	fileName, err := ZipFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	if fileName == "" {
		t.Fatal("empty file name")
	}

	fi, err := os.Lstat(fileName)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.Mode().IsRegular() {
		t.Fatalf("not a regular file: %s", fileName)
	}

	if fi.Size() == 0 {
		t.Fatalf("empty file: %s", fileName)
	}

	err = os.Remove(fileName)
	if err != nil {
		t.Fatal(err)
	}
}
