FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
COPY cmd ./cmd

RUN go build -o file-storage ./cmd/server

FROM alpine:3.20

WORKDIR /app

ENV PORT=8081
ENV DATA_DIR=/data/files

RUN mkdir -p ${DATA_DIR}

COPY --from=builder /app/file-storage /app/file-storage

EXPOSE 8081

VOLUME ["/data/files"]

CMD ["/app/file-storage"]


