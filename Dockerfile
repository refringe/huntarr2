FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
ARG TARGETOS
ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags="-s -w \
      -X main.version=${VERSION} \
      -X main.commit=${COMMIT} \
      -X main.date=${DATE}" \
    -o /bin/huntarr2 ./cmd/huntarr2

FROM alpine:3.24

LABEL org.opencontainers.image.title="Huntarr2" \
      org.opencontainers.image.description="Automated quality upgrade searches for *arr applications" \
      org.opencontainers.image.source="https://github.com/refringe/huntarr2" \
      org.opencontainers.image.licenses="AGPL-3.0"

RUN apk add --no-cache ca-certificates shadow su-exec tzdata && \
    addgroup -g 1000 huntarr2 && \
    adduser -u 1000 -G huntarr2 -D -h /config huntarr2 && \
    mkdir -p /config && \
    chown huntarr2:huntarr2 /config

COPY --from=build /bin/huntarr2 /bin/huntarr2
COPY --chmod=755 entrypoint.sh /entrypoint.sh

# ENCRYPTION_KEY defaults to a key generated and persisted in /config by entrypoint.sh when unset.
ENV PORT=9706
ENV LOG_LEVEL=info

VOLUME /config

EXPOSE 9706

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:9706/api/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
