## Stackdriver utilization log exporter
```shell
gcloud auth application-default login
go run exporter.go $PROJECT_ID $LAST_X_SECONDS
```