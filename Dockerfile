FROM golang:1.25-alpine AS builder

WORKDIR /app
ENV CGO_ENABLED=0 GOOS=linux

COPY go.mod go.sum ./
RUN go mod download 2>&1

COPY . .
RUN go build -o server ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /data
COPY --from=builder /app/server /usr/local/bin/server

EXPOSE 8080
ENTRYPOINT ["server"]
