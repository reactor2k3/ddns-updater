# syntax=docker/dockerfile:1
ARG GO_VERSION=1.26.10
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS base
WORKDIR /tmp/gobuild
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY pkg/ ./pkg/
COPY cmd/ ./cmd/
COPY internal/ ./internal/

FROM base AS test
ENV CGO_ENABLED=1
COPY readme/ ./readme/
COPY README.md ./README.md
ENTRYPOINT ["go", "test", "-race", "-count=1", "./..."]

FROM base AS build
RUN mkdir -p /tmp/data
ARG VERSION=development
ARG CREATED=unknown
ARG COMMIT=unknown
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X main.version=$VERSION -X main.date=$CREATED -X main.commit=$COMMIT" \
    -o app ./cmd/ddns-updater

FROM scratch
EXPOSE 8000
HEALTHCHECK --interval=60s --timeout=5s --start-period=10s --retries=2 CMD ["/updater/ddns-updater", "healthcheck"]
ARG UID=1000
ARG GID=1000
USER ${UID}:${GID}
WORKDIR /updater
ENTRYPOINT ["/updater/ddns-updater"]
COPY --from=build --chown=${UID}:${GID} /tmp/data /updater/data
ENV \
    # Core
    CONFIG= \
    PERIOD=5m \
    UPDATE_COOLDOWN_PERIOD=5m \
    PUBLICIP_FETCHERS=all \
    PUBLICIP_HTTP_PROVIDERS=all \
    PUBLICIPV4_HTTP_PROVIDERS=all \
    PUBLICIPV6_HTTP_PROVIDERS=all \
    PUBLICIP_DNS_PROVIDERS=all \
    PUBLICIP_DNS_TIMEOUT=3s \
    HTTP_TIMEOUT=10s \
    DATADIR=/updater/data \
    CONFIG_FILEPATH=/updater/data/config.json \
    RESOLVER_ADDRESS= \
    RESOLVER_TIMEOUT=5s \
    # Web UI
    SERVER_ENABLED=yes \
    LISTENING_ADDRESS=:8000 \
    ROOT_URL=/ \
    AUTH_ORIGIN=http://localhost:8000 \
    # Backup
    BACKUP_PERIOD=0 \
    BACKUP_DIRECTORY=/updater/data \
    # Other
    LOG_LEVEL=info \
    LOG_CALLER=hidden \
    SHOUTRRR_ADDRESSES= \
    SHOUTRRR_DEFAULT_TITLE="DDNS Updater" \
    TZ= \
    # UMASK left empty so it dynamically defaults to the OS current umask
    UMASK= \
    HEALTH_SERVER_ADDRESS=127.0.0.1:9999 \
    HEALTH_HEALTHCHECKSIO_BASE_URL=https://hc-ping.com \
    HEALTH_HEALTHCHECKSIO_UUID=
ARG VERSION=unknown
ARG CREATED="an unknown date"
ARG COMMIT=unknown
LABEL \
    org.opencontainers.image.authors="reactor2k3; original project by Quentin McGaw" \
    org.opencontainers.image.version=$VERSION \
    org.opencontainers.image.created=$CREATED \
    org.opencontainers.image.revision=$COMMIT \
    org.opencontainers.image.url="https://github.com/reactor2k3/ddns-updater" \
    org.opencontainers.image.documentation="https://github.com/reactor2k3/ddns-updater" \
    org.opencontainers.image.source="https://github.com/reactor2k3/ddns-updater" \
    org.opencontainers.image.title="ddns-updater" \
    org.opencontainers.image.description="Dynamic DNS updater with authenticated web management, passkeys and backups"
COPY --from=build --chown=${UID}:${GID} /tmp/gobuild/app /updater/ddns-updater
COPY LICENSE /updater/LICENSE
