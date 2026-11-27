# Vertexia web platform image (used by Cloudflare Containers via cloudflare/wrangler.jsonc).
# Only the main web binary is built here; ./cmd/gsm-agent runs on the gameserver host.

ARG GO_IMAGE=golang:1.25.14-alpine3.24@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59
ARG RUNTIME_IMAGE=alpine:3.24.4@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

# ---------- builder ----------
# Builder runs natively on the build host and cross-compiles a static linux/amd64 binary.
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOTOOLCHAIN=local

WORKDIR /src

# Module files first (fauxgl is a local replace target, so it must be present for go mod download).
COPY go.mod go.sum ./
COPY fauxgl/ ./fauxgl/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/vertexia .

# ---------- runtime ----------
# Cloudflare Containers run linux/amd64; build with --platform linux/amd64.
FROM ${RUNTIME_IMAGE} AS runtime

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 vertexia \
    && adduser -S -D -H -u 10001 -G vertexia -s /sbin/nologin vertexia

WORKDIR /app

COPY --from=builder --chown=root:root --chmod=0755 /out/vertexia /app/vertexia
COPY --chown=root:root views/ /app/views/
COPY --chown=root:root static/ /app/static/
COPY --chown=root:root assets/ /app/assets/

# Runtime-written dirs (dev mode; production writes to R2). Writable by the app user only.
RUN mkdir -p /app/uploads /app/static/uploads /app/static/renders /app/logs \
    && chown -R 10001:10001 /app/uploads /app/static/uploads /app/static/renders /app/logs

ENV PORT=8080 \
    APP_ENV=production

EXPOSE 8080

USER 10001:10001

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/app/vertexia"]
