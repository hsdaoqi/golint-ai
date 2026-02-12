# --- 第一阶段：构建 ---
FROM golang:1.24-alpine AS builder

# 必须安装 git，否则 go 可能会在某些依赖处理上报错
RUN apk add --no-cache git

WORKDIR /app

# 拷贝依赖
COPY go.mod go.sum ./
RUN go mod download

# 拷贝源码
COPY . .

# 【核心修正 1】：增加 -buildvcs=false。
# 告诉 Go 不要去翻代码仓库的版本信息，直接编译。
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /app/engine-bin ./cmd/golint-ai

# --- 第二阶段：运行 ---
FROM golang:1.24-alpine

# 运行环境也需要 git 和一些基础库
RUN apk add --no-cache git

# 【核心修正 2】：告诉 git 信任容器内的工作目录。
# 解决那个该死的 exit status 128 权限问题。
RUN git config --global --add safe.directory '*'

COPY --from=builder /app/engine-bin /usr/local/bin/golint-ai

RUN chmod +x /usr/local/bin/golint-ai

# 保持工作目录与 GitHub Action 挂载路径一致
WORKDIR /github/workspace

ENTRYPOINT ["/usr/local/bin/golint-ai"]