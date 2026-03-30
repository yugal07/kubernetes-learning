# ============================================================
# STAGE 1: Build — full Go toolchain, compiles our code
# ============================================================
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy dependency files first to leverage Docker layer caching
# (same idea as copying package.json before npm install)
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a static binary: no C deps, targets Linux
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# ============================================================
# STAGE 2: Runtime — minimal image with just the binary
# ============================================================
FROM alpine:latest

# Our app connects to MongoDB over TLS, so we need SSL certs at runtime
RUN apk add --no-cache ca-certificates

WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]