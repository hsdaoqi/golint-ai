# --- 第一阶段：构建 ---
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 编译工具本身
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /app/engine-bin ./cmd/golint-ai

# --- 第二阶段：运行 ---
FROM golang:1.24-alpine

RUN apk add --no-cache git

# 【核心修正 1】：强制全局禁用 Go 的 VCS 检查
# 这是解决 runtime 阶段（扫描代码时）报错的关键
ENV GOFLAGS="-buildvcs=false"

# 【核心修正 2】：双重保险，信任所有目录
RUN git config --global --add safe.directory '*'

COPY --from=builder /app/engine-bin /usr/local/bin/golint-ai

RUN chmod +x /usr/local/bin/golint-ai

# 设置工作目录
WORKDIR /github/workspace

ENTRYPOINT ["/usr/local/bin/golint-ai"]