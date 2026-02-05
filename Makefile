.PHONY: run test build

run:
	go run ./cmd/sendrec

test:
	go test ./...

build:
	go build -o bin/sendrec ./cmd/sendrec
