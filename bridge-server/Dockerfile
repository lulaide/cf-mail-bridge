FROM golang:1.25-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct

COPY . .

RUN go mod download

# GOOS=linux: 目标系统为 Linux
RUN CGO_ENABLED=0 GOOS=linux go build -o bridge-server .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/bridge-server .

EXPOSE 8888

ENTRYPOINT ["./bridge-server"]
