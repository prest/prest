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
ENV PREST_BUILD_PLUGINS=1
COPY --from=builder /bin/nc /bin/nc
RUN groupadd --system prest \
&& useradd --system --gid prest --home-dir /app --shell /usr/sbin/nologin prest
COPY --from=builder /workspace/prestd /app/prestd
COPY --from=builder /workspace/etc/entrypoint.sh /app/entrypoint.sh
COPY --from=builder --chown=prest:prest /workspace/lib /app/lib
COPY --from=builder /workspace/vendor /app/vendor
COPY --from=builder --chown=prest:prest /workspace/etc/plugin /app/plugin
RUN chown prest:prest /app
WORKDIR /app
USER prest
ENTRYPOINT ["sh", "/app/entrypoint.sh"]

# GoReleaser: prebuilt binary + extra_files only (no studio/ in context)
FROM registry.hub.docker.com/library/debian:bookworm AS release
RUN apt-get update && apt-get upgrade -y && \
    apt-get install --no-install-recommends -yq netcat-traditional && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd --system prest && \
    useradd --system --gid prest --home-dir /app --shell /usr/sbin/nologin prest
ENV PREST_BUILD_PLUGINS=0
COPY --from=builder /workspace/prestd /app/prestd
COPY --from=builder /workspace/etc/entrypoint.sh /app/entrypoint.sh
COPY --from=builder /workspace/lib /app/lib
COPY --from=builder /workspace/vendor /app/vendor
COPY --from=builder /workspace/etc/plugin /app/plugin
RUN chown -R prest:prest /app/lib /app/plugin
WORKDIR /app
USER prest
ENTRYPOINT ["sh", "/app/entrypoint.sh"]

FROM full
