.PHONY: test lint lint-full vet fmt boundary check security

test:
	go test ./... -count=1

lint:
	golangci-lint run ./...

lint-full:
	golangci-lint run --enable-all ./...

vet:
	go vet ./...

fmt:
	gofumpt -w .
	goimports -w -local github.com/outerstellar-hq/gouterstellar-platform .

boundary:
	@test -z "$$(go list -f '{{if eq .Name "main"}}{{.ImportPath}}{{end}}' ./...)"
	@test ! -d cmd
	@test ! -d internal
	@test ! -d extensions
	@test ! -d platform

check: boundary vet test lint

security:
	gosec ./...
