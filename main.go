package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-resty/resty/v2"
	"github.com/jxsl13/cli-config-boilerplate/cliconfig"
	"github.com/jxsl13/demo-uploader/config"
	"github.com/spf13/cobra"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cmd := NewRootCmd(ctx)
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func NewRootCmd(ctx context.Context) *cobra.Command {
	cli := &CLI{
		ctx: ctx,
		cfg: config.NewConfig(),
	}

	cmd := cobra.Command{
		Use: filepath.Base(os.Args[0]),
	}
	cmd.PreRunE = cli.PrerunE(&cmd)
	cmd.RunE = cli.RunE
	return &cmd
}

type CLI struct {
	ctx context.Context
	cfg config.Config
}

func (cli *CLI) PrerunE(cmd *cobra.Command) func(*cobra.Command, []string) error {
	parser := cliconfig.RegisterFlags(&cli.cfg, false, cmd)
	return func(cmd *cobra.Command, args []string) error {
		log.SetOutput(cmd.OutOrStdout()) // redirect log output to stdout
		return parser()                  // parse registered commands
	}
}

func (cli *CLI) RunE(cmd *cobra.Command, args []string) error {
	return Watch(cli.ctx, cli.cfg.WebhookUrl, cli.cfg.WatchFolder, cli.cfg.FileRegexp, cli.cfg.UploadTimeout)
}

func Watch(ctx context.Context, webhookUrl, watchPath string, regex *regexp.Regexp, uploadTimeout time.Duration) error {
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
	defer lwm.Close()

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

			if !regex.MatchString(event.Name) {
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
