SHELL := /bin/sh
BINARY := newsfall
VERSION := 0.1.1
LDFLAGS := -s -w

.PHONY: build test race snapshot release checksums clean

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags='$(LDFLAGS)' -o bin/$(BINARY) ./cmd/newsfall

test:
	test -z "$$(gofmt -l cmd internal)"
	go test ./...
	go vet ./...

race:
	go test -race ./...

snapshot: build
	mkdir -p dist
	./bin/$(BINARY) --demo --snapshot --width 150 --height 40 > dist/newsfall-demo.ansi.txt
	./bin/$(BINARY) --demo --snapshot --plain --width 110 --height 30 > dist/newsfall-demo.txt

release: test
	rm -rf dist
	mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags='$(LDFLAGS)' -o dist/$(BINARY)-darwin-arm64 ./cmd/newsfall
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags='$(LDFLAGS)' -o dist/$(BINARY)-darwin-amd64 ./cmd/newsfall
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='$(LDFLAGS)' -o dist/$(BINARY)-linux-amd64 ./cmd/newsfall
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='$(LDFLAGS)' -o dist/$(BINARY)-linux-arm64 ./cmd/newsfall
	cp README.md LICENSE dist/
	cp -R examples dist/examples
	cd dist && shasum -a 256 $(BINARY)-darwin-arm64 $(BINARY)-darwin-amd64 $(BINARY)-linux-amd64 $(BINARY)-linux-arm64 > SHA256SUMS

checksums:
	cd dist && shasum -a 256 $(BINARY)-darwin-arm64 $(BINARY)-darwin-amd64 $(BINARY)-linux-amd64 $(BINARY)-linux-arm64 > SHA256SUMS

clean:
	rm -rf bin dist
