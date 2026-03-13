FROM node:22-alpine AS frontend
WORKDIR /build
COPY web/admin/package.json web/admin/package-lock.json* web/admin/bun.lock* ./
RUN npm ci --ignore-scripts 2>/dev/null || npm install
COPY web/admin/ ./
RUN npm run build
# Output is in ../../embed/admin relative to web/admin
# But since WORKDIR is /build, it goes to /embed/admin

FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /embed/admin ./embed/admin
RUN CGO_ENABLED=0 go build -tags "with_utls,with_quic,with_gvisor,with_wireguard" \
    -ldflags "-s -w \
    -X github.com/FrankFMY/burrow/internal/shared.Version=$(git describe --tags --always 2>/dev/null || echo docker) \
    -X github.com/FrankFMY/burrow/internal/shared.Commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" \
    -o /burrow-server ./cmd/burrow-server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget && \
    addgroup -g 1000 burrow && \
    adduser -u 1000 -G burrow -s /bin/sh -D burrow && \
    mkdir -p /etc/burrow /var/lib/burrow && \
    chown -R burrow:burrow /etc/burrow /var/lib/burrow

COPY --from=builder /burrow-server /usr/local/bin/burrow-server

EXPOSE 443 8080 8443 8388
VOLUME ["/etc/burrow", "/var/lib/burrow"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

USER burrow
ENTRYPOINT ["burrow-server"]
CMD ["run", "--config", "/etc/burrow/burrow-server.json"]
