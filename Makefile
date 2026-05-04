.PHONY: build test lint lint-full vet fmt check generate clean migrate-up migrate-down dev seed security

build:
	go build -o bin/server.exe ./cmd/server

test:
	go test ./... -timeout 120s -count=1

lint:
	golangci-lint run ./...

lint-full:
	golangci-lint run --enable-all ./...

vet:
	go vet ./...

fmt:
	gofumpt -w .
	goimports -w -local github.com/rygel/gouterstellar-platform .

check: fmt vet lint
	@echo "All checks passed."

generate:
	sqlc generate

clean:
	rm -rf bin/

migrate-up:
	go run ./cmd/migrate

dev:
	APP_PROFILE=dev go run ./cmd/server

seed:
	go run ./cmd/seed

security:
	gosec ./...

build-seed:
	go build -o bin/seed.exe ./cmd/seed

build-migrate:
	go build -o bin/migrate.exe ./cmd/migrate
