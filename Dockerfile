# Build Stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy dependensi go.mod & go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copy source code lainnya
COPY . .

# Build binary Go static yang dioptimasi
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o gobaas main.go

# Run Stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary dari stage builder
COPY --from=builder /app/gobaas .

# Ekspos port default
EXPOSE 8080

# Jalankan program
CMD ["./gobaas"]
