# 服务端（Go）多阶段构建：runtime 内包含 citywall-server / seed / migrate 三个二进制。
# 容器启动由 docker-compose 的 entrypoint 执行「迁移 + seed + 启动」。

FROM golang:1.27-alpine AS builder
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/citywall-server ./cmd/server \
 && CGO_ENABLED=0 go build -o /out/seed ./cmd/seed \
 && CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate

FROM alpine:3.20
RUN adduser -D app && mkdir -p /app/data/storage && chown -R app /app/data
WORKDIR /app
COPY --from=builder /out/ /app/bin/
COPY migrations/ /app/migrations/
USER app
EXPOSE 8080
CMD ["/app/bin/citywall-server"]
