# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy everything first so go mod tidy can see all imports.
# Once you commit go.sum, swap to:
#   COPY go.mod go.sum ./
#   RUN go mod download
#   COPY . .
# for better layer caching.
COPY . .
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .

# ── Runtime stage ─────────────────────────────────────────────────────────────
# Static files and templates are embedded in the binary — no copies needed.
FROM alpine:3.21

RUN apk add --no-cache ca-certificates wget

WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 5000

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://localhost:5000/health || exit 1

CMD ["./server"]
