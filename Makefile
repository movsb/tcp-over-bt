.PHONY: device
device:
	GOOS=linux GOARCH=arm64 go build
.PHONY: host
host:
	go build
