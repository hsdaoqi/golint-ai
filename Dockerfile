# --- 第一阶段：构建 (Build) ---
# 使用 golang 官方镜像作为构建环境
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 拷贝依赖文件并下载
COPY go.mod go.sum ./
RUN go mod download

# 拷贝所有源代码
COPY . .

# 编译成名为 golint-ai 的二进制文件
# CGO_ENABLED=0 表示静态编译，不依赖宿主机的动态库
RUN CGO_ENABLED=0 GOOS=linux go build -o golint-ai ./cmd/golint-ai

# --- 第二阶段：运行 (Run) ---
# 既然我们的工具需要执行 `go build` 校验，所以基础镜像必须带 Go 环境
FROM golang:1.21-alpine

# 安装 git（很多 Go 项目依赖 git 下载包）
RUN apk add --no-cache git

# 从构建阶段拷贝编译好的二进制文件到当前镜像
COPY --from:builder /app/golint-ai /usr/local/bin/golint-ai/

# 设置执行权限
RUN chmod +x /usr/local/bin/golint-ai

# 定义容器启动时执行的指令
# 我们通过环境变量获取参数
ENTRYPOINT ["golint-ai"]