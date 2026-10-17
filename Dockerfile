# Build stage (modeled on agentgear: multi-stage Go + alpine, non-root runtime)
FROM golang:alpine AS builder

WORKDIR /app

# go-sqlite3 needs CGO; production compose uses Postgres but the binary still links sqlite for local/dev.
RUN apk add --no-cache git gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o linkgear ./cmd/linkgear/

# Runtime stage
FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata wget curl \
  && adduser -D -u 1000 linkgear \
  && mkdir -p /app/data \
  && chown -R linkgear:linkgear /app

COPY --from=builder /app/linkgear .

USER linkgear

ENV LINKGEAR_HTTP_ADDR=:8080 \
    LINKGEAR_PUBLIC_URL=http://localhost:8080 \
    LINKGEAR_DSN=sqlite:///app/data/linkgear.db \
    TZ=Asia/Shanghai

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --retries=3 --start-period=10s \
  CMD wget --quiet --tries=1 --spider http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["./linkgear"]
