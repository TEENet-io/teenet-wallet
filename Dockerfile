# ── Stage 1: Build frontend from submodule ────────────────
FROM node:20-alpine AS frontend

WORKDIR /app
COPY frontend-src/package.json frontend-src/package-lock.json* ./
RUN npm ci --silent
COPY frontend-src/ .
RUN npm run build

# ── Stage 2: Build Go backend ─────────────────────────────
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build
# Cache Go module downloads separately from source changes
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Replace frontend-src with built static files
COPY --from=frontend /app/dist ./frontend
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o wallet-app .

# ── Stage 3: Runtime image ────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache sqlite-libs ca-certificates tzdata

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /build/wallet-app .
COPY --from=builder /build/frontend ./frontend
COPY --from=builder /build/chains.json .

RUN mkdir -p /data && chown app:app /data

VOLUME /data

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/health || exit 1

ENV HOST=0.0.0.0 \
    PORT=8080 \
    DATA_DIR=/data \
    CONSENSUS_URL=http://localhost:8089 \
    FAUCET_URL=https://test.teenet.io/instance/faucet

USER app

ENTRYPOINT ["./wallet-app"]
