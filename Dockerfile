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

FROM alpine:3.21
RUN set -eu; \
    attempt=1; \
    while ! apk add --no-cache ca-certificates curl wget ffmpeg; do \
      if [ "$attempt" -ge 5 ]; then exit 1; fi; \
      echo "apk package install failed (attempt ${attempt}/5); retrying after backoff"; \
      sleep "$((attempt * 5))"; \
      attempt=$((attempt + 1)); \
    done
COPY --from=build /iptv-tunerr /usr/local/bin/iptv-tunerr
EXPOSE 5004
ENTRYPOINT ["iptv-tunerr"]
# Default: run tuner (refresh catalog, health check, serve). Override: e.g. docker run ... iptv-tunerr:local serve -addr :5004
CMD ["run", "-addr", ":5004"]
