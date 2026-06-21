# Stage 1: Build Frontend (Vue SPA)
FROM node:20-alpine AS frontend-builder
WORKDIR /app/studio
COPY studio/package*.json ./
RUN npm install
COPY studio/ ./
RUN npm run build

# Stage 2: Build Backend (Go)
FROM golang:1.21-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy compiled frontend assets into Go context
COPY --from=frontend-builder /app/studio/dist ./studio/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o gobaas main.go

# Stage 3: Run Stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=backend-builder /app/gobaas .
# Copy compiled frontend assets to run container
COPY --from=backend-builder /app/studio/dist ./studio/dist
EXPOSE 8080
CMD ["./gobaas"]
