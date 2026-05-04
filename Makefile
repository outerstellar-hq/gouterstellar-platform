.PHONY: build test lint generate clean migrate-up migrate-down dev seed

build:
	go build -o bin/server.exe ./cmd/server

test:
	go test ./... -timeout 120s -count=1

lint:
	golangci-lint run ./...

generate:
	sqlc generate

clean:
	rm -rf bin/

migrate-up:
	migrate -path migrations -database "postgres://outerstellar:outerstellar@localhost:5432/outerstellar?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://outerstellar:outerstellar@localhost:5432/outerstellar?sslmode=disable" down 1

dev:
	go run ./cmd/server

seed:
	go run ./cmd/seed
