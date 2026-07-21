.PHONY: build test vet check clean

BINARY := bin/njuprobe

build:
	go build -o $(BINARY) ./cmd/njuprobe

test:
	go test ./...

vet:
	go vet ./...

check: test vet

clean:
	rm -rf bin
