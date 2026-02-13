package checkers

import (
	"fmt"
	"go/ast"
	"golang.org/x/tools/go/analysis"
	"os"
	"regexp"
)

// 秘钥相关的关键词正则
var secretKeyRegex = regexp.MustCompile(`(?i)(api_key|password|passwd|secret|token|credential|access_id)`)

func ScanHardcodedSecrets(pass *analysis.Pass, f *ast.File) []Issue {
	var issues []Issue
	ast.Inspect(f, func(n ast.Node) bool {
		// 1. 寻找赋值语句：key := "..."
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for i, lhs := range as.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}

			// 检查变量名是否匹配秘钥关键词
			if secretKeyRegex.MatchString(id.Name) {
				if len(as.Rhs) > i {
					// 2. 检查右值是否是硬编码的常量字符串
					basic, ok := as.Rhs[i].(*ast.BasicLit)
					// 排除空字符串，且长度大于一定阈值（比如秘钥通常较长）
					if ok && len(basic.Value) > 5 {
						start := pass.Fset.Position(as.Pos())
						end := pass.Fset.Position(as.End())
						content, _ := os.ReadFile(start.Filename)

						issues = append(issues, Issue{
							Pos:      as.Pos(),
							End:      as.End(),
							VarName:  id.Name,
							Snippet:  string(content[start.Offset:end.Offset]),
							Message:  fmt.Sprintf("🛡️ 安全风险：变量 '%s' 疑似包含硬编码秘钥，建议移至环境变量", id.Name),
							Category: "HardcodedSecret",
						})
					}
				}
			}
		}
		return true
	})
	return issues
}
