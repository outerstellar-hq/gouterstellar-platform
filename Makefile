.PHONY: modules test race lint lint-full vet fmt boundary check security

modules:
	go mod verify
	go mod tidy -diff

test:
	go test ./... -count=1

race:
	go test -race ./... -count=1

lint:
	golangci-lint run --timeout=5m ./...

lint-full:
	golangci-lint run --default=all --timeout=5m ./...

vet:
	go vet ./...

fmt:
	gofumpt -w .
	goimports -w -local github.com/outerstellar-hq/gouterstellar-platform .

boundary:
	pwsh -NoProfile -File scripts/verify-library-boundary.ps1

check: modules boundary vet test lint

security:
	gosec ./...
