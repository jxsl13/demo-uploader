# demo-uploader

watch folder for new demos that match a specific regular expression and upload them to a discord webhook.
This utility can be used to upload any kind of file to a discord channel.


# building

```shell
go build .

# or

go install .

# or

go install github.com/jxsl13/demo-uploader@latest
```

# running

```shell
$ demo-uploader --help
Environment variables:
  WATCH_FOLDER      The folder to watch for file changes
  FILE_REGEX        The regex to match specific file names only (default: ".*\\.demo$")
  WEBHOOK_URL       Discord webhook url to upload the file to
  UPLOAD_TIMEOUT    how long to wait for the file to be untouched before uploading (default: "5m0s")
  SIZE_LIMIT        The maximum size of the zipped file to upload (e.g. MB, KB, MiB, KiB). Set to 0B to disable (default: "10MB")

Usage:
  demo-uploader [flags]

Flags:
  -c, --config string             .env config file path (or via env variable CONFIG)
      --file-regex string         The regex to match specific file names only (default ".*\\.demo$")
  -h, --help                      help for demo-uploader
      --size-limit string         The maximum size of the zipped file to upload (e.g. MB, KB, MiB, KiB). Set to 0B to disable (default "10MB")
      --upload-timeout duration   how long to wait for the file to be untouched before uploading (default 5m0s)
      --watch-folder string       The folder to watch for file changes
      --webhook-url string        Discord webhook url to upload the file to
```

```shell
$ demo-uploader --webhook-url <webhook-url> --watch-folder <folder> [--file-regex '.*\\.demo$'] [--upload-timeout 5m10s]
```

or preferrably with a config file in order not to leak your webhook url:

```dotenv
# mandatory
WEBHOOK_URL=<webhook-url>
WATCH_FOLDER=<folder>

# optional
FILE_REGEX=.*\.demo$
UPLOAD_TIMEOUT=5m0s
```

```shell
$ demo-uploader --config <config-file.env>
```
