package main

import (
	"golint-ai/pkg/analyzer"
	"golint-ai/pkg/config"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	// 0. 初始化配置
	config.Load()

	// 1. 定义 Cobra 根命令
	var rootCmd = &cobra.Command{
		Use:   "golint-ai",
		Short: "一款基于 AI 的工业级 Go 语言静态扫描工具",
		Run: func(cmd *cobra.Command, args []string) {
			// 2. 将官方的分析器包装进 CLI
			// singlechecker.Main 会自动处理：
			// - 递归扫描 ./...
			// - 输出结果格式化
			// - 并发性能优化
			singlechecker.Main(analyzer.Analyzer)
		},
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
