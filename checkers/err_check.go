package checkers

import (
	"fmt"
	"go/ast"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
	"os"
)

func ScanUnhandledError(pass *analysis.Pass, f *ast.File) []Issue {
	var issues []Issue
	ast.Inspect(f, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for _, lhs := range as.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok || id.Name == "_" {
				continue
			}

			typ := pass.TypesInfo.TypeOf(id)
			if typ != nil && typ.String() == "error" {
				if !isHandled(pass, id, f) {
					// 提取源码片段
					start := pass.Fset.Position(as.Pos())
					end := pass.Fset.Position(as.End())
					content, _ := os.ReadFile(start.Filename)

					issues = append(issues, Issue{
						Pos:      as.Pos(),
						End:      as.End(),
						VarName:  id.Name,
						Snippet:  string(content[start.Offset:end.Offset]),
						Message:  fmt.Sprintf("⚠️ 变量 %s 类型为 error 但未被 if 或 return 处理", id.Name),
						Category: "UnhandledError",
					})
				}
			}
		}
		return true
	})
	return issues
}

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
