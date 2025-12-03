FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
COPY cmd ./cmd

RUN go build -o api-gateway ./cmd/server

FROM alpine:3.20

WORKDIR /app

ENV PORT=8080
ENV FILE_STORAGE_URL="http://file-storage:8081"
ENV ANALYSIS_URL="http://file-analysis:8082"

COPY --from=builder /app/api-gateway /app/api-gateway

EXPOSE 8080

CMD ["/app/api-gateway"]


