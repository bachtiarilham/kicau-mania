FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/bootstrap-admin ./cmd/bootstrap-admin
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && addgroup -g 10001 app && adduser -D -u 10001 -G app app
WORKDIR /app
COPY --from=builder /out/ /usr/local/bin/
USER 10001:10001
ENV HTTP_ADDR=0.0.0.0:8091 LOG_DIR=/app/logs
EXPOSE 8091
STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/api"]
