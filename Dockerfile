FROM --platform=$BUILDPLATFORM alpine:3.23 AS management-asset

ARG MANAGEMENT_CENTER_RELEASE_API=https://api.github.com/repos/router-for-me/Cli-Proxy-API-Management-Center/releases/latest

RUN apk add --no-cache ca-certificates curl jq

RUN set -eu; \
    curl -fsSL -H "Accept: application/vnd.github+json" "${MANAGEMENT_CENTER_RELEASE_API}" -o /tmp/management-release.json; \
    asset_url="$(jq -r '.assets[] | select(.name == "management.html") | .browser_download_url' /tmp/management-release.json)"; \
    digest="$(jq -r '.assets[] | select(.name == "management.html") | (.digest // "")' /tmp/management-release.json)"; \
    test -n "${asset_url}"; \
    curl -fsSL "${asset_url}" -o /management.html; \
    case "${digest}" in \
        sha256:*) echo "${digest#sha256:}  /management.html" | sha256sum -c - ;; \
        ""|"null") true ;; \
        *) echo "unsupported management asset digest: ${digest}" >&2; exit 1 ;; \
    esac

FROM golang:1.26-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends build-essential git && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown
ARG TARGETOS=linux
ARG TARGETARCH

RUN target_arch="${TARGETARCH:-$(go env GOARCH)}"; \
    CGO_ENABLED=1 GOOS="${TARGETOS:-linux}" GOARCH="${target_arch}" go build -buildvcs=false -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" -o ./CLIProxyAPI ./cmd/server/

FROM debian:bookworm

RUN apt-get update && apt-get install -y --no-install-recommends tzdata ca-certificates && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /CLIProxyAPI/static

COPY --from=builder ./app/CLIProxyAPI /CLIProxyAPI/CLIProxyAPI
COPY --from=management-asset /management.html /CLIProxyAPI/static/management.html

COPY config.example.yaml /CLIProxyAPI/config.example.yaml

WORKDIR /CLIProxyAPI

EXPOSE 8317

ENV TZ=Asia/Shanghai

RUN cp /usr/share/zoneinfo/${TZ} /etc/localtime && echo "${TZ}" > /etc/timezone

CMD ["./CLIProxyAPI"]
