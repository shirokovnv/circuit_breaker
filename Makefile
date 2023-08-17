#!/usr/bin/make

.PHONY : help init format build run test
.DEFAULT_GOAL : help

EXAMPLE := example/main.go

# HELP =================================================================================================================
# This will output the help for each task
# thanks to https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help: ## Display this help screen
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

init: ### Init project
	go mod download

format: ### Format go files
	gofmt -l .

build: ### Build example
	go build $(EXAMPLE)

run: ### Run example
	go run $(EXAMPLE)

test: ### Run tests
	go test -v ./...