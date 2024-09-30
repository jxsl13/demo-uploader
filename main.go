package main

import (
	"archive/zip"
	"compress/flate"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-resty/resty/v2"
)

func main() {
	watchPath := flag.String("watch-folder", "", "use this flag in order to specify the path to watch")
	regex := flag.String("file-regex", ".*\\.demo$", "use this flag in order to match specific file names only")
	webhookUrl := flag.String("webhook-url", "", "use this flag in order to send a webhook request when a file is modified")
	uploadTimeout := flag.Duration("upload-timeout", 5*time.Minute, "use this flag in order to specify the upload timeout, how long to wait after last file write before uploading")
	flag.Parse()

	if *regex == "" {
		*regex = ".*"
	}

	webhookURI, err := url.ParseRequestURI(*webhookUrl)
	if err != nil {
		log.Fatalf("Invalid webhook URL: %s", *webhookUrl)
	}

	if *watchPath == "" {
		log.Fatalf("Please specify a path to watch")
	}

	fi, err := os.Lstat(*watchPath)
	if err != nil {
		log.Fatalf("Error while trying to access path: %s", *watchPath)
	}

	if !fi.IsDir() {
		log.Fatalf("Path is not a directory: %s", *watchPath)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err = Watch(ctx, webhookURI.String(), *watchPath, *regex, *uploadTimeout)
	if err != nil {
		log.Fatalf("Error while watching: %s", err)
	}
}

func Watch(ctx context.Context, webhookUrl, watchPath string, regex string, uploadTimeout time.Duration) error {
	re, err := regexp.Compile(regex)
	if err != nil {
		return fmt.Errorf("invalid regex: %s: %w", regex, err)
	}

	var lwm *LastWriteMap
	lwm = NewLastWriteMap(ctx, func(filePath string, deadline time.Time) error {
		fi, err := os.Lstat(filePath)
		if err != nil {
			return fmt.Errorf("error while trying to access file: %s: %w", filePath, err)
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("not a regular file: %s", filePath)
		}

		now := time.Now()
		diff := now.Sub(fi.ModTime())
		if diff >= uploadTimeout {
			// upload if file has not been touched in the last minute
			err = Upload(webhookUrl, filePath)
			if err != nil {
				return fmt.Errorf("error while uploading file: %s: %w", filePath, err)
			}
		} else {
			delay := uploadTimeout - diff
			log.Println("file", filePath, "was modified too recently, will upload in", delay)
			lwm.Set(filePath, now.Add(delay))
		}
		return nil
	})

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Add a path.
	err = watcher.Add(watchPath)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("context done, closing watcher")
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return errors.New("watcher events channel closed unexpectedly")
			}

			if strings.HasSuffix(event.Name, ".zip") {
				continue
			}

			if !re.MatchString(event.Name) {
				log.Println("skipping not-matching file:", event.Name)
				continue
			}

			if event.Has(fsnotify.Write) {
				log.Println("adding file to watch queue:", event.Name, "will be uploaded in", uploadTimeout)
				lwm.Set(event.Name, time.Now().Add(uploadTimeout))
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return errors.New("watcher errors channel closed unexpectedly")
			}
			log.Println("error:", err)
		}
	}
}

func Upload(webhookUrl, filePath string) error {
	zippedName, err := ZipFile(filePath)
	if err != nil {
		return fmt.Errorf("error while zipping file: %s: %w", filePath, err)
	}
	defer os.Remove(zippedName)

	log.Println("uploading zipped file:", zippedName)

	resp, err := resty.New().
		R().
		SetHeader("Content-Type", "multipart/form-data").
		SetFile(filepath.Base(zippedName), zippedName).
		Post(webhookUrl)
	if err != nil {
		return fmt.Errorf("error while uploading file: %s: %w", filePath, err)
	}

	if resp.IsError() {
		return fmt.Errorf("error while uploading file: %s: %s", filePath, resp.Status())
	}

	return nil
}
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

type LastWriteMap struct {
	ctx context.Context
	mu  sync.Mutex
	m   map[string]time.Time

	drained bool
	timer   *time.Timer
}

func NewLastWriteMap(ctx context.Context, do func(key string, deadline time.Time) error) *LastWriteMap {
	lwm := &LastWriteMap{
		ctx:     ctx,
		m:       make(map[string]time.Time),
		timer:   time.NewTimer(0),
		drained: false,
	}

	<-lwm.timer.C
	lwm.drained = true

	go lwm.routine(do)
	return lwm
}

func (lwm *LastWriteMap) peekNextDeadline() (key string, deadline time.Time, ok bool) {
	if len(lwm.m) == 0 {
		return "", time.Time{}, false
	}

	var (
		minDeadline time.Time
		minKey      string
	)

	for key, deadline := range lwm.m {
		if deadline.Before(minDeadline) || minDeadline.IsZero() {
			minDeadline = deadline
			minKey = key
		}
	}
	return minKey, minDeadline, true
}

func (lwm *LastWriteMap) popNextDeadline() (key string, deadline time.Time, ok bool) {
	key, deadline, ok = lwm.peekNextDeadline()
	if ok {
		delete(lwm.m, key)
	}
	return key, deadline, ok
}

func (lwm *LastWriteMap) Set(key string, deadline time.Time) {
	lwm.mu.Lock()
	defer lwm.mu.Unlock()

	lwm.m[key] = deadline
	lwm.resetTimer()
}

func (lwm *LastWriteMap) routine(do func(key string, deadline time.Time) error) {
	for {
		select {
		case <-lwm.ctx.Done():
			log.Println("context done, stopping routine")
			return
		case <-lwm.timer.C:
			func() {
				lwm.mu.Lock()

				lwm.drained = true
				k, v, ok := lwm.popNextDeadline()
				if !ok {
					log.Println("no dealines, sleeping...")
					lwm.mu.Unlock()
					return
				}
				lwm.mu.Unlock()

				// do not lock here, cuz we may be manipulating the map
				// from within that function
				log.Println("processing:", k)
				err := do(k, v)
				if err != nil {
					log.Println("error while processing:", err)
				}

				lwm.mu.Lock()
				defer lwm.mu.Unlock()
				d := lwm.resetTimer()
				if d > 0 {
					log.Println("next deadline in:", d)
				} else {
					log.Println("no more deadlines left, sleeping...")
				}
			}()
		}
	}
}

func (lwm *LastWriteMap) resetTimer() time.Duration {
	_, v, ok := lwm.peekNextDeadline()
	if !ok {
		return 0
	}
	d := time.Until(v)
	resetTimer(lwm.timer, d, &lwm.drained)
	return d
}
