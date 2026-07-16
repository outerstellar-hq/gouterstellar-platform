ARG GO_VERSION=1.26.2
FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=0 GOMAXPROCS=4
RUN go build -p=4 -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
    && go build -p=4 -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate \
    && go build -p=4 -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM alpine:3.23

RUN addgroup -S outerstellar && adduser -S -G outerstellar outerstellar
WORKDIR /app

COPY --from=build /out/server /out/migrate /out/seed /app/
COPY config /app/config
COPY static /app/static

USER outerstellar
EXPOSE 8080
ENTRYPOINT ["/app/server"]
