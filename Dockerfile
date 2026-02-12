# --- 第一阶段：构建 ---
FROM golang:1.24-alpine AS builder

WORKDIR /app

# 拷贝依赖
COPY go.mod go.sum ./
RUN go mod download

# 拷贝源码
COPY . .

# 【关键修改】：将输出文件起名为 engine-bin，避免与目录名 golint-ai 冲突
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/engine-bin ./cmd/golint-ai

# --- 第二阶段：运行 ---
FROM golang:1.24-alpine

RUN apk add --no-cache git

# 【关键修改】：显式地将 builder 阶段的文件拷贝为最终的可执行文件
COPY --from=builder /app/engine-bin /usr/local/bin/golint-ai

# 赋予执行权限
RUN chmod +x /usr/local/bin/golint-ai

# 使用绝对路径运行
ENTRYPOINT ["/usr/local/bin/golint-ai"]