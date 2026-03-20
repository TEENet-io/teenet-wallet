FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build
COPY . .
RUN go mod download \
    && CGO_ENABLED=1 GOOS=linux go build -o wallet-app .

# ─── Runtime image ────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache sqlite-libs ca-certificates tzdata

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /build/wallet-app .
COPY --from=builder /build/frontend ./frontend
COPY --from=builder /build/chains.json .

RUN mkdir -p /data && chown app:app /data

EXPOSE 8080

ENV HOST=0.0.0.0 \
    PORT=8080 \
    DATA_DIR=/data \
    CONSENSUS_URL=http://localhost:8089

USER app

ENTRYPOINT ["./wallet-app"]
