package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"time"
)

func NewConfig() Config {
	return Config{
		WatchFolder:   "",
		FileRegex:     ".*\\.demo$",
		WebhookUrl:    "",
		UploadTimeout: 5 * time.Minute,
	}
}

type Config struct {
	WatchFolder   string `koanf:"watch.folder" description:"The folder to watch for file changes"`
	FileRegex     string `koanf:"file.regex" description:"The regex to match specific file names only"`
	FileRegexp    *regexp.Regexp
	WebhookUrl    string        `koanf:"webhook.url" description:"Discord webhook url to upload the file to"`
	UploadTimeout time.Duration `koanf:"upload.timeout" description:"how long to wait for the file to be untouched before uploading"`
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
	return nil
}
