FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG TARGETOS TARGETARCH TARGETVARIANT VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY . .
RUN set -eu; \
    goarm=""; \
    if [ "${TARGETARCH}" = "arm" ] && [ -n "${TARGETVARIANT:-}" ]; then \
      goarm="${TARGETVARIANT#v}"; \
    fi; \
    CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" GOARM="${goarm}" \
      go build -mod=vendor \
      -ldflags="-X main.Version=${VERSION}" \
      -o /iptv-tunerr ./cmd/iptv-tunerr

FROM debian:bookworm-slim
RUN set -eu; \
    attempt=1; \
    while ! apt-get -o Acquire::Retries=3 update \
      || ! DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates curl ffmpeg wget; do \
      if [ "$attempt" -ge 3 ]; then \
        echo "Unable to install container runtime packages from Debian mirrors" >&2; \
        exit 1; \
      fi; \
      echo "Debian package install failed (attempt ${attempt}/3); retrying after backoff"; \
      rm -rf /var/lib/apt/lists/*; \
      sleep "$((attempt * 5))"; \
      attempt=$((attempt + 1)); \
    done; \
    rm -rf /var/lib/apt/lists/*
COPY --from=build /iptv-tunerr /usr/local/bin/iptv-tunerr
EXPOSE 5004
ENTRYPOINT ["iptv-tunerr"]
# Default: run tuner (refresh catalog, health check, serve). Override: e.g. docker run ... iptv-tunerr:local serve -addr :5004
CMD ["run", "-addr", ":5004"]
