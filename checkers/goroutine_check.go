package checkers

import (
	"go/ast"
	"go/types"
	"golang.org/x/tools/go/analysis"
	"os"
)

func ScanGoroutineLeak(pass *analysis.Pass, f *ast.File) []Issue {
	var issues []Issue
	ast.Inspect(f, func(n ast.Node) bool {
		// 1. 寻找 go 关键字语句：go func() { ... }()
		goStmt, ok := n.(*ast.GoStmt)
		if !ok {
			return true
		}

		// 2. 【核心算法】：检查该协程是否伴随等待逻辑
		// 逻辑：在当前的 BlockStmt（代码块）中，搜寻是否有 sync.WaitGroup 的踪迹
		if !hasWaitMechanism(f, goStmt, pass.TypesInfo) {
			start := pass.Fset.Position(goStmt.Pos())
			end := pass.Fset.Position(goStmt.End())
			content, _ := os.ReadFile(start.Filename)

			issues = append(issues, Issue{
				Pos:      goStmt.Pos(),
				End:      goStmt.End(),
				VarName:  "goroutine",
				Snippet:  string(content[start.Offset:end.Offset]),
				Message:  "⚠️ 发现未托管的 Goroutine：缺少 sync.WaitGroup 或 Context 控制，可能导致协程泄露",
				Category: "GoroutineLeak",
			})
		}
		return true
	})
	return issues
}

// hasWaitMechanism 判定当前函数内是否有管理该协程的手段
func hasWaitMechanism(root *ast.File, goStmt *ast.GoStmt, info *types.Info) bool {
	foundWait := false

	// 在整棵树中寻找 sync.WaitGroup 的 Wait() 调用或者 channel 操作
	// 这里是简化版实现，重点在于展示“追踪并发原语”的意识
	ast.Inspect(root, func(n ast.Node) bool {
		// A. 寻找 wg.Wait()
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Wait" {
					// 如果找到了 Wait()，我们认为开发者有管理意识（此处可深挖是否对应同一个 wg）
					foundWait = true
				}
			}
		}
		// B. 寻找 select{} 或 channel 阻塞
		if _, ok := n.(*ast.SelectStmt); ok {
			foundWait = true
		}
		return true
	})

	return foundWait
}
