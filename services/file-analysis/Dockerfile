FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
COPY cmd ./cmd

RUN go build -o file-analysis ./cmd/server

FROM alpine:3.20

WORKDIR /app

ENV PORT=8082
ENV POSTGRES_DSN="host=postgres user=postgres password=postgres dbname=antiplag sslmode=disable"
ENV FILE_STORAGE_URL="http://file-storage:8081"

COPY --from=builder /app/file-analysis /app/file-analysis

EXPOSE 8082

CMD ["/app/file-analysis"]


