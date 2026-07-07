BINARY := kubectl-neighbours

.PHONY: build test lint fmt snapshot clean

build:
	go build -o $(BINARY) .

test:
	go test -race -coverprofile=coverage.out ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf dist/ $(BINARY) coverage.out
