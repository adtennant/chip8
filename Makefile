.PHONY : build fmt lint test tidy

build:
	go build -v -o bin/chip8 .

fmt:
	gofmt -l -w ./

lint:
	golangci-lint run
	staticcheck ./...

test:
	go clean -testcache; go test ./... -v

tidy:
	go mod tidy
