# ── Build stage ────────────────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/open-coreui ./cmd/openwebui

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/open-coreui /app/open-coreui

# Data directory for SQLite / uploads
RUN mkdir -p /app/data

EXPOSE 8081

CMD ["/app/open-coreui"]
