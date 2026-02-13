package checkers

import (
	"go/token"
)

// Issue 描述一个被发现的代码缺陷
type Issue struct {
	Pos      token.Pos // 错误发生的位置
	End      token.Pos // 错误结束的位置
	VarName  string    // 涉事变量名
	Snippet  string    // 代码原始片段
	Message  string    // 给用户的提示信息
	Category string    // 缺陷类别：NilPointer / UnhandledError
}
