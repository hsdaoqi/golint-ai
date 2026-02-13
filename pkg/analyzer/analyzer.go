package analyzer

import (
	"bufio"
	"fmt"
	"github.com/hsdaoqi/golint-ai/checkers"
	"github.com/hsdaoqi/golint-ai/pkg/repairer"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"

	"log"
	"os"
	"strings"
	"sync"
)

// FixMode 全局开关
var FixMode bool
var consoleMu sync.Mutex // 交互锁：防止多个 Worker 同时在终端说话

type fixTask struct {
	pass  *analysis.Pass
	issue checkers.Issue
}

var Analyzer = &analysis.Analyzer{
	Name:     "errfix",
	Doc:      "errfix analyzes Go code and suggests AI-based fixes",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	taskChan := make(chan fixTask, 100)
	var wg sync.WaitGroup

	// 启动 Worker
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range taskChan {
				processTask(t)
			}
		}()
	}

	for _, f := range pass.Files {
		// 聚合所有检查器的结果
		issues := checkers.ScanUnhandledError(pass, f)
		issues = append(issues, checkers.ScanNilPointer(pass, f)...)
		issues = append(issues, checkers.ScanResourceLeak(pass, f)...)

		for _, iss := range issues {
			taskChan <- fixTask{pass: pass, issue: iss}
		}
	}

	close(taskChan)
	wg.Wait()
	return nil, nil
}

func processTask(t fixTask) {
	// 1. 请求 AI 获取建议
	fixCode, err := repairer.GetFix(t.issue.VarName, t.issue.Snippet, "", t.issue.Category)
	if err != nil {
		log.Printf("AI Error: %v", err)
		return
	}

	// 2. 如果是 FixMode，进入交互逻辑
	if FixMode {
		consoleMu.Lock() // 抢占控制台
		defer consoleMu.Unlock()

		fmt.Printf("\n发现问题: %s\n", t.issue.Message)
		fmt.Printf("原始代码: %s\n", t.issue.Snippet)
		fmt.Printf("AI 修复建议: \n%s\n", fixCode)
		fmt.Print("是否应用修复并写入文件? (y/n): ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) == "y" {
			applyFixToFile(t.pass, t.issue, fixCode)
			fmt.Println("已成功修复并保存。")
		} else {
			fmt.Println("已跳过。")
		}
	} else {
		// Scan 模式只打印建议
		fmt.Printf("\n[%s] 建议: %s\n", t.issue.Category, fixCode)
		t.pass.Report(analysis.Diagnostic{
			Pos:     t.issue.Pos,
			Message: t.issue.Message,
		})
	}
}

// applyFixToFile 真正的“手术刀”：修改磁盘文件
func applyFixToFile(pass *analysis.Pass, iss checkers.Issue, fixCode string) {
	// 1. 获取文件位置信息
	pos := pass.Fset.Position(iss.Pos)
	end := pass.Fset.Position(iss.End)

	// 2. 读取原文件
	content, _ := os.ReadFile(pos.Filename)

	// 3. 拼接新内容
	newContent := append([]byte{}, content[:pos.Offset]...)
	newContent = append(newContent, []byte(fixCode)...)
	newContent = append(newContent, content[end.Offset:]...)

	// 4. 写回磁盘
	os.WriteFile(pos.Filename, newContent, 0644)
}
