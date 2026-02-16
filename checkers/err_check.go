package checkers

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
	"os"
)

func ScanUnhandledError(pass *analysis.Pass, f *ast.File) []Issue {
	var issues []Issue

	allAssigns := getAllAssigns(f)

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

			obj := pass.TypesInfo.ObjectOf(id)
			//typ := pass.TypesInfo.TypeOf(id)
			if obj == nil || !isErrorType(obj.Type()) {
				continue
			}
			//获取当前赋值的结束位置
			currentAssignEnd := as.End()
			//获取下一次被赋值的位置
			nextAssignPos := getNextAssignmentPos(id.Name, currentAssignEnd, allAssigns)

			if !isHandledInInterval(pass, f, obj, currentAssignEnd, nextAssignPos) {
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
		return true
	})
	return issues
}

// isHandledRobustly 工业级处理判定逻辑
func isHandledInInterval(pass *analysis.Pass, root *ast.File, obj types.Object, start, end token.Pos) bool {
	// 遍历该 error 对象在全文件的所有“引用点”
	for ident, useObj := range pass.TypesInfo.Uses {
		if useObj != obj {
			continue
		}

		// 技巧：如果使用的位置就在赋值语句本身（LHS），跳过
		// 【关键算法】：位置过滤
		// 1. 使用位置必须在当前赋值之后
		// 2. 如果存在下一次重赋值，使用位置必须在下一次重赋值之前
		if ident.Pos() <= start || (end != token.NoPos && ident.Pos() >= end) {
			continue
		}
		// 【新增工业级判定】：是否被存储到了结构体、Map或切片中（视为逃逸/透传）
		if isEscaped(root, ident) {
			return true
		}
		// 判定 A：是否在 if 语句的【条件表达式】中进行了比较？
		// 如：if err != nil
		if isUsedInComparison(root, ident) {
			return true
		}

		// 判定 B：是否被直接 return 了？
		if isUsedInReturn(root, ident) {
			return true
		}

		// 判定 C：是否被送进了处理函数？
		// 如：panic(err), log.Fatal(err), fmt.Errorf("...%w", err)
		if isUsedInHandler(root, ident) {
			return true
		}
	}
	return false
}

// isEscaped 检查变量是否被塞进了结构体字段或容器
func isEscaped(root *ast.File, ident *ast.Ident) bool {
	path, _ := astutil.PathEnclosingInterval(root, ident.Pos(), ident.End())
	for _, node := range path {
		// 1. 检查是否在结构体初始化中：FixResult{ Error: err }
		if _, ok := node.(*ast.KeyValueExpr); ok {
			return true
		}
		// 2. 检查是否在数组/切片初始化中：[]error{ err }
		if _, ok := node.(*ast.CompositeLit); ok {
			return true
		}
	}
	return false
}

// isUsedInComparison 判断 err 是否参与了 nil 比较
func isUsedInComparison(root *ast.File, ident *ast.Ident) bool {
	path, _ := astutil.PathEnclosingInterval(root, ident.Pos(), ident.End())
	for _, node := range path {
		// 检查是否在 BinaryExpr 中 (err != nil)
		if bin, ok := node.(*ast.BinaryExpr); ok {
			if bin.Op.String() == "!=" || bin.Op.String() == "==" {
				return true
			}
		}
		// 检查是否直接作为 if 的条件 (if err { ... } 在 Go 中虽不合法但需兼容类型转换)
		if ifStmt, ok := node.(*ast.IfStmt); ok && ifStmt.Cond == ident {
			return true
		}
	}
	return false
}

// isUsedInReturn 判断是否直接返回
func isUsedInReturn(root *ast.File, ident *ast.Ident) bool {
	path, _ := astutil.PathEnclosingInterval(root, ident.Pos(), ident.End())
	for _, node := range path {
		if _, ok := node.(*ast.ReturnStmt); ok {
			return true
		}
	}
	return false
}

// isUsedInHandler 判断是否被 panic 或常用日志库消费
func isUsedInHandler(root *ast.File, ident *ast.Ident) bool {
	path, _ := astutil.PathEnclosingInterval(root, ident.Pos(), ident.End())
	for _, node := range path {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			continue
		}
		// 1. 内置函数 panic(err)
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "panic" {
			return true
		}
		// 2. 常见的日志/错误包装函数 log.Fatal, fmt.Errorf
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			switch sel.Sel.Name {
			case "Fatal", "Fatalf", "Panic", "Panicf", "Errorf", "Wrap":
				return true
			}
		}
	}
	return false
}
func isErrorType(t types.Type) bool {
	if t == nil {
		return false
	}
	return t.String() == "error" || t.Underlying().String() == "error"
}

// 辅助函数：获取同一个变量名下一次被赋值的位置
func getNextAssignmentPos(name string, after token.Pos, allAssigns []*ast.AssignStmt) token.Pos {
	for _, as := range allAssigns {
		if as.Pos() > after {
			for _, lhs := range as.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
					return as.Pos()
				}
			}
		}
	}
	return token.NoPos // 后面没有重赋值了
}

func getAllAssigns(f *ast.File) []*ast.AssignStmt {
	var res []*ast.AssignStmt
	ast.Inspect(f, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok {
			res = append(res, as)
		}
		return true
	})
	return res
}
