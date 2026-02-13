package checkers

import (
	"go/ast"
	"go/types"
	"golang.org/x/tools/go/analysis"
	"os"
)

var dbMethodRegex = map[string]bool{
	"Query": true, "Exec": true, "QueryRow": true, "Select": true,
}

func ScanSQLInjection(pass *analysis.Pass, f *ast.File) []Issue {
	var issues []Issue
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// 1. 识别数据库操作
		if dbMethodRegex[sel.Sel.Name] {
			if len(call.Args) > 0 {
				firstArg := call.Args[0]

				// 2. 【核心升级】：不仅检查表达式，还检查变量来源
				if isTainted(firstArg, pass.TypesInfo) {
					start := pass.Fset.Position(call.Pos())
					end := pass.Fset.Position(call.End())
					content, _ := os.ReadFile(start.Filename)

					issues = append(issues, Issue{
						Pos:      call.Pos(),
						End:      call.End(),
						VarName:  sel.Sel.Name,
						Snippet:  string(content[start.Offset:end.Offset]),
						Message:  "🛡️ SQL 注入风险：检测到污点变量流入数据库查询，请使用参数化查询改写",
						Category: "SQLInjection",
					})
				}
			}
		}
		return true
	})
	return issues
}

// isTainted 污点分析核心逻辑：判断一个表达式是否“不洁”
func isTainted(expr ast.Expr, info *types.Info) bool {
	// 情况 A：直接拼接或 Sprintf (例如直接传入 fmt.Sprintf)
	if isDangerousSQLString(expr) {
		return true
	}

	// 情况 B：回溯变量定义 (针对 query := fmt.Sprintf 模式)
	if id, ok := expr.(*ast.Ident); ok {
		// 找到该变量定义时的身份证 (Object)
		obj := info.Uses[id]
		if obj == nil {
			return false
		}

		// 在 AST 中寻找该变量的定义赋值语句
		// 这是一个简化的局部搜索
		tainted := false
		// 我们去寻找定义这个变量的那个声明或赋值
		if def, ok := info.Defs[id]; ok && def != nil {
			// 这里逻辑较复杂，我们采用更通用的方法：查找同一个作用域内的赋值
		}

		// 工业级做法：通过递归寻找赋值语句的右手边 (RHS)
		ast.Inspect(id.Obj.Decl.(ast.Node), func(n ast.Node) bool {
			if as, ok := n.(*ast.AssignStmt); ok {
				for _, rhs := range as.Rhs {
					if isDangerousSQLString(rhs) {
						tainted = true
					}
				}
			}
			return true
		})
		return tainted
	}
	return false
}

func isDangerousSQLString(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.CallExpr:
		if fun, ok := e.Fun.(*ast.SelectorExpr); ok {
			if x, ok := fun.X.(*ast.Ident); ok && x.Name == "fmt" && fun.Sel.Name == "Sprintf" {
				return true
			}
		}
	case *ast.BinaryExpr:
		if e.Op.String() == "+" {
			return true
		}
	}
	return false
}
