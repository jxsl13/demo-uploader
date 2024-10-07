package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/alecthomas/units"
)

func NewConfig() Config {
	return Config{
		WatchFolder:   "",
		FileRegex:     ".*\\.demo$",
		WebhookUrl:    "",
		UploadTimeout: 5 * time.Minute,
		SizeLimit:     "10MB",
	}
}

type Config struct {
	WatchFolder    string `koanf:"watch.folder" description:"The folder to watch for file changes"`
	FileRegex      string `koanf:"file.regex" description:"The regex to match specific file names only"`
	FileRegexp     *regexp.Regexp
	WebhookUrl     string        `koanf:"webhook.url" description:"Discord webhook url to upload the file to"`
	UploadTimeout  time.Duration `koanf:"upload.timeout" description:"how long to wait for the file to be untouched before uploading"`
	SizeLimit      string        `koanf:"size.limit" description:"The maximum size of the zipped file to upload (e.g. MB, KB, MiB, KiB). Set to 0B to disable"`
	SizeLimitBytes int64
}

func (cfg *Config) Validate() error {
	if cfg.FileRegex == "" {
		cfg.FileRegex = ".*"
	}

	re, err := regexp.Compile(cfg.FileRegex)
	if err != nil {
		return fmt.Errorf("invalid regex: %s", cfg.FileRegex)
	}
	cfg.FileRegexp = re

	webhookURI, err := url.ParseRequestURI(cfg.WebhookUrl)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %s", cfg.WebhookUrl)
	}
	cfg.WebhookUrl = webhookURI.String()

	if cfg.WatchFolder == "" {
		return fmt.Errorf("please specify a directory path to watch")
	}

	fi, err := os.Lstat(cfg.WatchFolder)
	if err != nil {
		return fmt.Errorf("error while trying to access path: %s", cfg.WatchFolder)
	}

	if !fi.IsDir() {
		return fmt.Errorf("path is not a directory: %s", cfg.WatchFolder)
	}

	if b, err := units.ParseMetricBytes(cfg.SizeLimit); err == nil {
		cfg.SizeLimitBytes = int64(b)
	} else if b, err := units.ParseBase2Bytes(cfg.SizeLimit); err == nil {
		cfg.SizeLimitBytes = int64(b)
	} else {
		return fmt.Errorf("invalid size limit: %s, must be KB, MB, KiB or MiB", cfg.SizeLimit)
	}

	return nil
}
