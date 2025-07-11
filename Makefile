.PHONY: all build clean test

all: build

build:
	go build -o bin/xoa-jsonrpc ./cmd/xoa-jsonrpc/

clean:
	rm -rf bin/

test:
	go test ./...
