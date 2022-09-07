PWD := $(shell pwd)
GOPATH := $(shell go env GOPATH)

all: verifiers test

getdeps: 
	@echo "Installing golint" && go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.42.0

verifiers: getdeps vet fmt lint cyclo

vet:
	@echo "Running $@"
	@go vet -atomic -bool -copylocks -nilfunc -printf -rangeloops -unreachable -unsafeptr -unusedresult ./...

fmt:
	@echo "Running $@"
	@gofmt -d .

lint:
	@echo "Running $@"
	@${GOPATH}/bin/golangci-lint run

test: 
	@echo "Running Tests"