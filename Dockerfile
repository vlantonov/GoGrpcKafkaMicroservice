# ── Builder ───────────────────────────────────────────────────────────────────
FROM golang:1.27-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o /app/server ./cmd/server

# ── Final ─────────────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/server /app/server

EXPOSE 50051 8080

CMD ["/app/server"]
