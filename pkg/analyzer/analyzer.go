package analyzer

import (
	"bufio"
	"fmt"
	"github.com/hsdaoqi/golint-ai/checkers"
	"github.com/hsdaoqi/golint-ai/pkg/repairer"
	"go/token"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
)

// FixMode 控制是仅扫描还是交互式修复
var FixMode bool

// AggregatedIssue 聚合了同一个位置的所有缺陷
type AggregatedIssue struct {
	Pos        token.Pos
	End        token.Pos
	VarName    string
	Snippet    string
	Categories []string
	Messages   []string
	Filename   string
}

// FixResult 存储 AI 的生成结果
type FixResult struct {
	Agg   *AggregatedIssue
	Patch string
	Error error
}

var Analyzer = &analysis.Analyzer{
	Name:     "errfix",
	Doc:      "工业级 AI 自动化修复引擎 (支持多缺陷聚合与逆序修复)",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		// 1. 调用所有检查器收集原始 Issues
		var rawIssues []checkers.Issue
		rawIssues = append(rawIssues, checkers.ScanUnhandledError(pass, f)...)
		rawIssues = append(rawIssues, checkers.ScanNilPointer(pass, f)...)
		rawIssues = append(rawIssues, checkers.ScanResourceLeak(pass, f)...)
		rawIssues = append(rawIssues, checkers.ScanHardcodedSecrets(pass, f)...)
		rawIssues = append(rawIssues, checkers.ScanSQLInjection(pass, f)...)
		if len(rawIssues) == 0 {
			continue
		}

		// 2. 【核心算法】：按位置(Pos)聚合缺陷
		// 解决“同一行代码修复两次”的 Bug
		issueMap := make(map[token.Pos]*AggregatedIssue)
		for _, iss := range rawIssues {
			posInfo := pass.Fset.Position(iss.Pos)
			if existing, found := issueMap[iss.Pos]; found {
				existing.Categories = append(existing.Categories, iss.Category)
				existing.Messages = append(existing.Messages, iss.Message)
			} else {
				issueMap[iss.Pos] = &AggregatedIssue{
					Pos:        iss.Pos,
					End:        iss.End,
					VarName:    iss.VarName,
					Snippet:    iss.Snippet,
					Categories: []string{iss.Category},
					Messages:   []string{iss.Message},
					Filename:   posInfo.Filename,
				}
			}
		}

		// 将 Map 转为 List 方便后续处理
		var aggregatedList []*AggregatedIssue
		for _, agg := range issueMap {
			aggregatedList = append(aggregatedList, agg)
		}

		// 3. 【并行层】：并发向 AI 申请修复方案
		results := make([]FixResult, len(aggregatedList))
		var wg sync.WaitGroup
		for i, agg := range aggregatedList {
			wg.Add(1)
			go func(idx int, target *AggregatedIssue) {
				defer wg.Done()
				// 将多个缺陷类型拼接，告诉 AI 一次性修好
				categoryDesc := strings.Join(target.Categories, " 且 ")
				patch, err := repairer.GetFix(target.VarName, target.Snippet, "", categoryDesc)
				results[idx] = FixResult{Agg: target, Patch: patch, Error: err}
			}(i, agg)
		}
		wg.Wait()

		// 4. 【排序层】：按 Pos 倒序排列 (从文件末尾往开头修)
		// 解决“修复后偏移量失效”的 Bug
		sort.Slice(results, func(i, j int) bool {
			return results[i].Agg.Pos > results[j].Agg.Pos
		})

		// 5. 【交互与输出层】：根据模式执行
		for _, res := range results {
			if res.Error != nil {
				log.Printf("AI 修复失败 [%s]: %v", res.Agg.VarName, res.Error)
				continue
			}

			if FixMode {
				// 修复模式：独占式交互
				handleFixInteraction(pass, res)
			} else {
				// 扫描模式：仅打印和汇报
				handleScanOutput(pass, res)
			}
		}
	}
	return nil, nil
}

// handleFixInteraction 处理 fix 命令的交互逻辑
func handleFixInteraction(pass *analysis.Pass, res FixResult) {
	fmt.Printf("\n" + strings.Repeat("=", 60))
	fmt.Printf("\n缺陷位置: %s:%d", res.Agg.Filename, pass.Fset.Position(res.Agg.Pos).Line)
	fmt.Printf("\n缺陷类别: %s", strings.Join(res.Agg.Categories, " & "))
	fmt.Printf("\n修复建议: \n%s", res.Patch)
	fmt.Printf("\n" + strings.Repeat("-", 60))
	fmt.Print("\n是否应用此修复并写入文件? (y/n): ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(input)) == "y" {
		applyFixToFile(pass, res.Agg, res.Patch)
		fmt.Println("已保存。")
	} else {
		fmt.Println("已跳过。")
	}
}

// handleScanOutput 处理 scan 命令的输出逻辑
func handleScanOutput(pass *analysis.Pass, res FixResult) {
	// 在控制台打印带颜色的建议（方便预览）
	fmt.Printf("\n[%s] 在 %s:%d 发现缺陷。AI 建议: \n%s\n",
		strings.Join(res.Agg.Categories, "&"),
		res.Agg.Filename,
		pass.Fset.Position(res.Agg.Pos).Line,
		res.Patch)

	// 同时向框架汇报，这样可以使用 go vet 标准输出
	pass.Report(analysis.Diagnostic{
		Pos:     res.Agg.Pos,
		Message: fmt.Sprintf("[%s] %s", strings.Join(res.Agg.Categories, "&"), strings.Join(res.Agg.Messages, "; ")),
	})
}

// applyFixToFile 物理修改文件
func applyFixToFile(pass *analysis.Pass, agg *AggregatedIssue, fixCode string) {
	pos := pass.Fset.Position(agg.Pos)
	end := pass.Fset.Position(agg.End)

	content, err := os.ReadFile(pos.Filename)
	if err != nil {
		log.Printf("无法读取文件: %v", err)
		return
	}

	// 构造新内容：前 + 补丁 + 后
	newContent := append([]byte{}, content[:pos.Offset]...)
	newContent = append(newContent, []byte(fixCode)...)
	newContent = append(newContent, content[end.Offset:]...)

	err = os.WriteFile(pos.Filename, newContent, 0644)
	if err != nil {
		log.Printf("写入失败: %v", err)
	}
}
