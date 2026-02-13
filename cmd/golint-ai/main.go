package main

import (
	"fmt"
	"github.com/hsdaoqi/golint-ai/pkg/analyzer"
	"github.com/hsdaoqi/golint-ai/pkg/config"
	"github.com/spf13/cobra"
	"golang.org/x/tools/go/analysis/singlechecker"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "golint-ai",
	Short: "GoLint-AI: AI-Powered Static Analysis Tool",
}

// scan 模式：只打印建议，不修改文件
var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "仅扫描代码并给出修复建议",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		analyzer.FixMode = false // 设置为非修复模式
		os.Args = append([]string{os.Args[0]}, args...)
		singlechecker.Main(analyzer.Analyzer)
	},
}

// fix 模式：发现问题后询问用户是否写入
var fixCmd = &cobra.Command{
	Use:   "fix [path]",
	Short: "交互式扫描并修复代码",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		analyzer.FixMode = true // 开启修复模式
		os.Args = append([]string{os.Args[0]}, args...)
		singlechecker.Main(analyzer.Analyzer)
	},
}

func init() {
	config.Load()
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(fixCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
