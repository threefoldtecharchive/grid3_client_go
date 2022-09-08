PWD := $(shell pwd)
GOPATH := $(shell go env GOPATH)

all: verifiers test

getdeps: 
	@echo "Installing golint" && go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.42.0

verifiers: getdeps vet fmt lint 

vet:
	@echo "Running $@"
	@go vet -atomic -bool -copylocks -nilfunc -printf -rangeloops -unreachable -unsafeptr -unusedresult ./...

fmt:
	@echo "Running $@"
	@gofmt -d .

lint:
	@echo "Running $@"
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.26.0 golangci-lint run

test: 
	@echo "Running Tests"