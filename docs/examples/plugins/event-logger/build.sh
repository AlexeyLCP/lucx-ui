#/bin/sh
# static linux build: CGO_ENABLED=0 GOOS=linux GOARCH=<amd64|arm64> go build -o event-logger .
# package: zip with manifest.json + the built binary entrypoint
