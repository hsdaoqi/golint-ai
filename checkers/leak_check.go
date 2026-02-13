package checkers

import (
	"fmt"
	"go/ast"
	"go/types"
	"golang.org/x/tools/go/analysis"
	"os"
)

func ScanResourceLeak(pass *analysis.Pass, f *ast.File) []Issue {
	var issues []Issue
	ast.Inspect(f, func(n ast.Node) bool {
		// 寻找赋值语句 f, _ := os.Open(...)
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for _, lhs := range as.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}

			// 语义分析：检查变量类型是否有 Close() 方法
			typ := pass.TypesInfo.TypeOf(id)
			if isCloser(typ) {
				if !hasDeferClose(f, id, pass.TypesInfo) {
					start := pass.Fset.Position(as.Pos())
					end := pass.Fset.Position(as.End())
					content, _ := os.ReadFile(start.Filename)
					issues = append(issues, Issue{
						Pos:      id.Pos(),
						End:      as.End(),
						VarName:  id.Name,
						Snippet:  string(content[start.Offset:end.Offset]),
						Message:  fmt.Sprintf("🚨 发现潜在资源泄露：变量 %s 未显式关闭", id.Name),
						Category: "ResourceLeak",
					})
				}
			}
		}
		return true
	})
	return issues
}

// 判断类型是否实现了 io.Closer 接口
func isCloser(t types.Type) bool {
	if t == nil {
		return false
	}
	// 查找该类型及其指针类型是否拥有 Close 方法
	m, _, _ := types.LookupFieldOrMethod(t, true, nil, "Close")
	return m != nil
}

// 检查函数内是否有 defer x.Close()
func hasDeferClose(root *ast.File, id *ast.Ident, info *types.Info) bool {
	found := false
	obj := info.Defs[id]
	ast.Inspect(root, func(n ast.Node) bool {
		d, ok := n.(*ast.DeferStmt)
		if !ok {
			return true
		}

		// 检查 defer 后面跟的是不是函数调用 x.Close()
		call, ok := d.Call.Fun.(*ast.SelectorExpr)
		if ok && call.Sel.Name == "Close" {
			if ident, ok := call.X.(*ast.Ident); ok {
				if info.Uses[ident] == obj {
					found = true
				}
			}
		}
		return true
	})
	return found
}
