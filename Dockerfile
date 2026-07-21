# syntax=docker/dockerfile:1

# --- Studio frontend ---
FROM node:24-bookworm AS studio
WORKDIR /workspace/studio
RUN corepack enable
COPY studio/package.json studio/pnpm-lock.yaml studio/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY studio/ ./
RUN pnpm build

# --- Go binary ---
FROM registry.hub.docker.com/library/golang:1.26 AS builder
WORKDIR /workspace
COPY . .
COPY --from=studio /workspace/internal/studio/dist /workspace/internal/studio/dist
ARG VERSION
ARG COMMIT
ARG DATE
ENV GOOS=linux
ENV CGO_ENABLED=1
# jwx v4 imports encoding/json/v2, gated behind GOEXPERIMENT=jsonv2 on Go 1.26.
ENV GOEXPERIMENT=jsonv2
RUN apt-get update && apt-get upgrade -y && \
    apt-get install --no-install-recommends -yq netcat-traditional && \
    rm -rf /var/lib/apt/lists/* && \
    if [ -f go.mod ]; then \
      go mod vendor && \
      go build -ldflags "-s -w -X github.com/prest/prest/v2/helpers.Version=${VERSION} -X github.com/prest/prest/v2/helpers.Commit=${COMMIT} -X github.com/prest/prest/v2/helpers.Date=${DATE}" -o prestd cmd/prestd/main.go; \
    fi

# Full-repo build (default for docker build .)
# Needs go to compile the plugin system
FROM registry.hub.docker.com/library/golang:1.26 AS full
RUN apt-get update && apt-get upgrade -y && rm -rf /var/lib/apt/lists/*
ENV CGO_ENABLED=1
# jwx v4 imports encoding/json/v2, gated behind GOEXPERIMENT=jsonv2 on Go 1.26.
ENV GOEXPERIMENT=jsonv2
ENV PREST_BUILD_PLUGINS=1
COPY --from=builder /bin/nc /bin/nc
COPY --from=builder /workspace/prestd /bin/prestd
COPY --from=builder /workspace/etc/entrypoint.sh /app/entrypoint.sh
COPY --from=builder /workspace/lib /app/lib
COPY --from=builder /workspace/etc/plugin /app/plugin
WORKDIR /app
ENTRYPOINT ["sh", "/app/entrypoint.sh"]

# GoReleaser: prebuilt binary + extra_files only (no studio/ in context)
FROM registry.hub.docker.com/library/golang:1.26 AS release
RUN apt-get update && apt-get upgrade -y && \
    apt-get install --no-install-recommends -yq netcat-traditional && \
    rm -rf /var/lib/apt/lists/*
ENV CGO_ENABLED=1
# jwx v4 imports encoding/json/v2, gated behind GOEXPERIMENT=jsonv2 on Go 1.26.
ENV GOEXPERIMENT=jsonv2
ENV PREST_BUILD_PLUGINS=1
COPY prestd /bin/prestd
COPY etc/entrypoint.sh /app/entrypoint.sh
COPY lib /app/lib
COPY etc/plugin /app/plugin
WORKDIR /app
ENTRYPOINT ["sh", "/app/entrypoint.sh"]

FROM full
