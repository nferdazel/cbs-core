# Multi-stage Dockerfile for Go Core API
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/bin/core-api ./cmd/server

# Production image
FROM alpine:3.21

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Jakarta

COPY --from=builder /app/bin/core-api /app/core-api

EXPOSE 8080

USER 1000:1000

ENTRYPOINT ["/app/core-api"]
