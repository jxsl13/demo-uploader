# demo-uploader

watch folder for new demos that match a specific regular expression and upload them to a discord webhook.


# building

```shell
go build .
```

# running

```shell
./demo-uploader --webhook-url <webhook-url> --watch-folder <folder> [--file-regex '.*\\.demo$'] [--upload-timeout 5m10s]
```

or preferrably with a config file in order not to leak your webhook url:

```dotenv
WEBHOOK_URL=<webhook-url>
WATCH_FOLDER=<folder>
FILE_REGEX=.*\.demo$
UPLOAD_TIMEOUT=5m10s
```

```shell
./demo-uploader --config <config-file.env>
```
