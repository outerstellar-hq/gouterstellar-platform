ARG GO_VERSION=1.26.2
FROM golang:${GO_VERSION}-alpine AS build

ARG BUILD_DATE=local
ARG BUILD_NUMBER=dev
ARG COMMIT_SHA=

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=0 GOMAXPROCS=4
RUN build_flags="-s -w -X github.com/outerstellar-hq/gouterstellar-platform/platform/buildinfo.buildDate=${BUILD_DATE} -X github.com/outerstellar-hq/gouterstellar-platform/platform/buildinfo.buildNumber=${BUILD_NUMBER} -X github.com/outerstellar-hq/gouterstellar-platform/platform/buildinfo.commitSHA=${COMMIT_SHA}" \
    && go build -p=4 -trimpath -ldflags="${build_flags}" -o /out/server ./cmd/server \
    && go build -p=4 -trimpath -ldflags="${build_flags}" -o /out/migrate ./cmd/migrate \
    && go build -p=4 -trimpath -ldflags="${build_flags}" -o /out/seed ./cmd/seed

FROM alpine:3.24

RUN addgroup -S outerstellar && adduser -S -G outerstellar outerstellar
WORKDIR /app

COPY --from=build /out/server /out/migrate /out/seed /app/
COPY config /app/config
COPY static /app/static

USER outerstellar
EXPOSE 8080
ENTRYPOINT ["/app/server"]
