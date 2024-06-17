generate:
	go generate ./...
lint:
	golangci-lint run --fix
test:
	go test -race -cover ./...
release:
	goreleaser build --snapshot
