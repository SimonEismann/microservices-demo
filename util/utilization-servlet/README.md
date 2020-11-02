Reads the CPU utilization from linux systems and provides a tcp server to serve them. Whenever a line is received, the most recent measurement probe is sent. Port: 22442.

## Build Binary
```shell script
#macOS
env GOOS=linux GOARCH=amd64 go build -v

#windows
set GOOS=linux
set GOARCH=amd64
go build -v
```