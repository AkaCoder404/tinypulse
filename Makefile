.PHONY: build run test clean

BINARY=tinypulse

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY) ./cmd/tinypulse

run:
	go run ./cmd/tinypulse

test:
	go test -v ./...

clean:
	rm -f $(BINARY) *.db *.db-shm *.db-wal
