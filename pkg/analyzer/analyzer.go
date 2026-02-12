package analyzer

import (
	"fmt"
	"go/ast"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/astutil"
	"golint-ai/pkg/repairer"
	"log"
	"os"
	"sync"
)

// 1. 定义任务包，包含所有需要的“树”信息和“语义”信息
type fixTask struct {
	pass     *analysis.Pass
	fileRoot *ast.File       // 当前变量所属的整棵树的根（用于溯源）
	as       *ast.AssignStmt // 赋值语句节点
	id       *ast.Ident      // 变量名节点
	snippet  string          // 提取出来的源码文本
}

var Analyzer = &analysis.Analyzer{
	Name:     "errfix",
	Doc:      "高并发 AI 自动化修复引擎",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	log.SetFlags(0) // 去掉 log 默认的时间前缀，让输出更整洁
	log.Printf("🚀 [GoLint-AI] 开始分析路径: %v", pass.Pkg.Path())
	// 创建一个缓冲区为 100 的管道（ACM 里的 Queue）
	taskChan := make(chan fixTask, 100)
	var wg sync.WaitGroup

	// --- 第一步：启动并发 Worker 池 ---
	workerCount := 5 // 限制并发，防止 API 频率限制 (Rate Limit)
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for task := range taskChan {
				// 执行耗时的 AI + 验证逻辑
				processFix(task, workerID)
			}
		}(i)
	}

	// --- 第二步：生产者逻辑（扫描所有文件） ---
	for _, f := range pass.Files {
		// 这里增加一行：看看它扫描了哪些文件
		log.Printf("📑 扫描文件: %s", pass.Fset.Position(f.Pos()).Filename)
		// 这里的 f 就是当前正在扫描的文件根节点
		ast.Inspect(f, func(n ast.Node) bool {
			// 找赋值语句: a, err := ...
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}

			for _, lhs := range as.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || id.Name == "_" {
					continue
				}

				// 语义检查：判断是否为 error 且未被处理
				if pass.TypesInfo.TypeOf(id).String() == "error" && !isHandled(pass, id, f) {

					// 提取源码片段用于 AI 修复
					start := pass.Fset.Position(as.Pos())
					end := pass.Fset.Position(as.End())
					content, _ := os.ReadFile(start.Filename)
					snippet := string(content[start.Offset:end.Offset])

					// 包装任务，发送给 Worker
					taskChan <- fixTask{
						pass:     pass,
						fileRoot: f, // 传递当前文件的根，用于后续溯源
						as:       as,
						id:       id,
						snippet:  snippet,
					}
				}
			}
			return true
		})
	}

	// --- 第三步：优雅关闭 ---
	close(taskChan) // 告诉 Workers 没有新任务了
	wg.Wait()       // 阻塞等待，直到 5 个 Worker 全部处理完
	// 强制刷新输出流，保证 CI 能够捕获所有日志
	os.Stdout.Sync()
	os.Stderr.Sync()
	return nil, nil
}

// processFix 是真正的“重体力活”：AI 修复 + 编译验证
func processFix(t fixTask, workerID int) {
	log.Printf("[Worker %d] 正在处理变量: %s...\n", workerID, t.id.Name)

	// 1. 第一次 AI 修复
	fixCode, err := repairer.GetFix(t.id.Name, t.snippet, "")
	if err != nil {
		return
	}

	// 2. 编译校验 (Verifier)
	// (此处逻辑简化：在完整版中，你需要将 fixCode 拼入原文件并调用 ValidatePatch)

	log.Printf("✨ [Worker %d] 成功为 %s 生成建议: %s\n", workerID, t.id.Name, fixCode)

	// 3. 汇报结果
	t.pass.Report(analysis.Diagnostic{
		Pos:     t.id.Pos(),
		Message: fmt.Sprintf("⚠️ 变量 %s 未被处理", t.id.Name),
		SuggestedFixes: []analysis.SuggestedFix{
			{
				Message: "应用 AI 修复方案",
				TextEdits: []analysis.TextEdit{
					{
						Pos:     t.as.Pos(),
						End:     t.as.End(),
						NewText: []byte(fixCode),
					},
				},
			},
		},
	})
}

// --- 以下是辅助判定函数（保持不变，依赖 astutil 和 path 溯源） ---

func isHandled(pass *analysis.Pass, id *ast.Ident, fileRoot *ast.File) bool {
	obj := pass.TypesInfo.Defs[id]
	if obj == nil {
		return false
	}

	for ident, useObj := range pass.TypesInfo.Uses {
		if useObj == obj {
			if isWithinIf(fileRoot, ident) || isWithinReturn(fileRoot, ident) {
				return true
			}
		}
	}
	return false
}

func isWithinIf(root *ast.File, ident *ast.Ident) bool {
	path, _ := astutil.PathEnclosingInterval(root, ident.Pos(), ident.End())
	for _, node := range path {
		if _, ok := node.(*ast.IfStmt); ok {
			return true
		}
	}
	return false
}

func isWithinReturn(root *ast.File, ident *ast.Ident) bool {
	path, _ := astutil.PathEnclosingInterval(root, ident.Pos(), ident.End())
	for _, node := range path {
		if _, ok := node.(*ast.ReturnStmt); ok {
			return true
		}
	}
	return false
}
