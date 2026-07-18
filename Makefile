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
	pwsh -NoProfile -File scripts/verify-library-boundary.ps1

check: boundary vet test lint

security:
	gosec ./...
