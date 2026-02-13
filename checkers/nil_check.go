package checkers

import (
	"fmt"
	"go/ast"
	"go/types"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
	"os"
)

func ScanNilPointer(pass *analysis.Pass, f *ast.File) []Issue {
	var issues []Issue
	ast.Inspect(f, func(n ast.Node) bool {
		// 1. 寻找常见的 ptr, err := func() 模式
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) < 2 {
			return true
		}

		var ptrId, errId *ast.Ident
		// 寻找左侧：一个是普通类型，一个是 error
		for _, lhs := range as.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}
			typ := pass.TypesInfo.TypeOf(id)
			if typ == nil {
				continue
			}
			if typ.String() == "error" {
				errId = id
			} else {
				ptrId = id // 潜在的可能为空的指针
			}
		}

		if ptrId != nil && errId != nil {
			// 2. 检查在此赋值语句后的代码块中，是否存在对 ptrId 的解引用（如 ptr.Field）
			// 且这种解引用发生在对 errId 的 if 检查之前
			if isRiskBeforeCheck(f, ptrId, errId, pass.TypesInfo) {
				start := pass.Fset.Position(as.Pos())
				end := pass.Fset.Position(as.End())
				content, _ := os.ReadFile(start.Filename)

				issues = append(issues, Issue{
					Pos:      ptrId.Pos(),
					End:      as.End(),
					VarName:  ptrId.Name,
					Snippet:  string(content[start.Offset:end.Offset]),
					Message:  fmt.Sprintf("🚨 空指针风险：在检查 %s 之前使用了可能为 nil 的变量 %s", errId.Name, ptrId.Name),
					Category: "NilPointer",
				})
			}
		}
		return true
	})
	return issues
}

// isRiskBeforeCheck 判定是否存在“先使用后检查”的风险
func isRiskBeforeCheck(root *ast.File, ptr, err *ast.Ident, info *types.Info) bool {
	// 1. 找到 ptr 定义时所在的路径（从根到节点的完整路径栈）
	path, _ := astutil.PathEnclosingInterval(root, ptr.Pos(), ptr.End())

	// 2. 找到包含该赋值语句的“语句 (Stmt)”和“代码块 (BlockStmt)”
	var startStmt ast.Stmt
	var parentBlock *ast.BlockStmt

	for _, node := range path {
		if s, ok := node.(ast.Stmt); ok && startStmt == nil {
			startStmt = s // 比如：f, err := os.Open(...)
		}
		if b, ok := node.(*ast.BlockStmt); ok {
			parentBlock = b // 包含该语句的 { ... } 块
			break
		}
	}

	if parentBlock == nil || startStmt == nil {
		return false
	}

	// 3. 寻找起始语句在块中的索引位置
	startIndex := -1
	for i, stmt := range parentBlock.List {
		if stmt == startStmt {
			startIndex = i
			break
		}
	}

	// 4. 【核心算法】：从定义处开始，向后扫描后续的所有语句
	for i := startIndex + 1; i < len(parentBlock.List); i++ {
		currStmt := parentBlock.List[i]

		// 风险 A：是否在没有检查 err 的情况下解引用了 ptr？
		// 比如出现了 ptr.Name() 或 ptr.Field
		if isDereferenced(currStmt, ptr, info) {
			return true // 发现风险！
		}

		// 安全点：是否遇到了对 err 的检查？
		// 比如出现了 if err != nil { ... }
		if isErrorChecked(currStmt, err, info) {
			return false // 已安全检查，后续不再有当前级别的空指针风险
		}
	}

	return false
}

// 辅助函数：判断该语句中是否解引用了指定的指针
func isDereferenced(n ast.Node, ptr *ast.Ident, info *types.Info) bool {
	found := false
	obj := info.Defs[ptr] // 获取 ptr 的身份证
	if obj == nil {
		obj = info.Uses[ptr]
	}

	ast.Inspect(n, func(node ast.Node) bool {
		// 检查 X.Sel 形式，比如 ptr.Field 或 ptr.Method()
		if sel, ok := node.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				if info.Uses[id] == obj {
					found = true
				}
			}
		}
		return true
	})
	return found
}

// 辅助函数：判断该语句是否是对 err 的有效检查
func isErrorChecked(n ast.Node, errIdent *ast.Ident, info *types.Info) bool {
	found := false
	errObj := info.Defs[errIdent]
	if errObj == nil {
		errObj = info.Uses[errIdent]
	}

	// 寻找 if err != nil
	if ifStmt, ok := n.(*ast.IfStmt); ok {
		ast.Inspect(ifStmt.Cond, func(condNode ast.Node) bool {
			if id, ok := condNode.(*ast.Ident); ok {
				if info.Uses[id] == errObj {
					found = true
				}
			}
			return true
		})
	}
	return found
}
