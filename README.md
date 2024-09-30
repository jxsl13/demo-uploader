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