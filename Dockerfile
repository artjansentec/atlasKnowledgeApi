# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S atlas \
    && adduser -S -G atlas atlas \
    && mkdir -p /data/storage \
    && chown -R atlas:atlas /data

COPY --from=builder /out/api /usr/local/bin/api

USER atlas

EXPOSE 8080

VOLUME ["/data/storage"]

HEALTHCHECK --interval=15s --timeout=5s --start-period=25s --retries=5 \
    CMD wget -qO- http://127.0.0.1:8080/api/v1/health >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/api"]
