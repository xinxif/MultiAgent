# Agent 多阶段构建（单镜像：server/worker/cli 按参数切换）
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGET=server
RUN CGO_ENABLED=0 go build -o /out/app ./cmd/${TARGET}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/app ./app
COPY configs ./configs
ENTRYPOINT ["./app"]
