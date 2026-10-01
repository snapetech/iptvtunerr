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
	alpine_version="$(cut -d. -f1,2 /etc/alpine-release)"; \
	install_from() { \
	  mirror="$1"; \
	  printf '%s\n' \
	    "${mirror}/v${alpine_version}/main" \
	    "${mirror}/v${alpine_version}/community" > /etc/apk/repositories; \
	  attempt=1; \
	  while ! apk add --no-cache ca-certificates curl wget ffmpeg; do \
	    if [ "$attempt" -ge 3 ]; then return 1; fi; \
	    echo "apk package install failed on ${mirror} (attempt ${attempt}/3); retrying after backoff"; \
	    sleep "$((attempt * 5))"; \
	    attempt=$((attempt + 1)); \
	  done; \
	}; \
	if ! install_from "https://dl-cdn.alpinelinux.org/alpine"; then \
	  fallback_ok=0; \
	  for mirror in \
	    "https://mirrors.edge.kernel.org/alpine" \
	    "https://us.mirror.ionos.com/linux/distributions/alpine" \
	    "https://plug-mirror.rcac.purdue.edu/alpine" \
	    "https://mirror.csclub.uwaterloo.ca/alpine"; do \
	    echo "Primary Alpine mirror unavailable; trying official mirror ${mirror}"; \
	    if install_from "${mirror}"; then \
	      fallback_ok=1; \
	      break; \
	    fi; \
	  done; \
	  if [ "${fallback_ok}" -ne 1 ]; then \
	    echo "All configured official Alpine mirrors failed" >&2; \
	    exit 1; \
	  fi; \
	fi
COPY --from=build /iptv-tunerr /usr/local/bin/iptv-tunerr
EXPOSE 5004
ENTRYPOINT ["iptv-tunerr"]
# Default: run tuner (refresh catalog, health check, serve). Override: e.g. docker run ... iptv-tunerr:local serve -addr :5004
CMD ["run", "-addr", ":5004"]
