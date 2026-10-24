# Multi-stage build for kyrecovery-server
FROM golang:1.27.3-alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Release builds pass the tag's version (X.Y.Z or X.Y.Z.N); other builds report internal/appversion/VERSION.
# -trimpath keeps the build directory out of the binary, so the same commit, Go version and
# VERSION give the same bytes wherever it is built (docs/VERIFYING-RELEASES.md).
ARG VERSION=
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X github.com/Busnes-app/kyrecovery-server/internal/appversion.release=${VERSION}" \
    -o kyrecovery cmd/kyrecovery/main.go

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
RUN apk --no-cache add ca-certificates tzdata su-exec

# KyRecovery needs no privilege beyond its own data directory, so it does not get
# any at runtime. The uid is fixed so a bind-mounted ./data can be chowned to
# match; docker-entrypoint.sh does that chown (needs root) then drops to this
# user before the app itself ever runs. /app and its executables stay
# root-owned so the app user cannot replace what root executes next boot;
# the account gets no home there, only /app/data is app-writable.
RUN addgroup -g 10001 -S kyrecovery && \
    adduser -u 10001 -S -G kyrecovery -H -h /nonexistent kyrecovery

WORKDIR /app
ENV KYRECOVERY_DATA_DIR=/app/data
COPY --from=builder /app/kyrecovery /app/kyrecovery
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod 0755 /app/docker-entrypoint.sh /app/kyrecovery && \
    mkdir -p /app/data && chown kyrecovery:kyrecovery /app/data

EXPOSE 8095
VOLUME ["/app/data"]

# serve is the default, not the only command: `docker run <image> audit` and
# `docker run <image> help` have to reach the CLI, so the subcommand is CMD.
ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["serve", "--port", "8095", "--data-dir", "/app/data"]
