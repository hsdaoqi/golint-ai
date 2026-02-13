package repairer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/hsdaoqi/golint-ai/pkg/config"
	"io"
	"net/http"
	"strings"
)

// LLM 交互协议结构体
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type AIResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

func GetFix(varName, codeSnippet, contextErr, combinedCheckers string) (string, error) {
	// 1. 定义缺陷字典
	allHints := map[string]string{
		"UnhandledError":  "【指令1：错误处理】添加标准的 'if err != nil' 逻辑，严禁忽略错误。",
		"NilPointer":      "【指令2：空指针防护】在解引用前必须进行 nil 检查，防止运行时宕机。",
		"ResourceLeak":    "【指令3：资源释放】使用 'defer' 显式调用 Close() 方法，防止内存或文件句柄泄露。",
		"HardcodedSecret": "【指令4：脱敏处理】将硬编码秘钥改为从 os.Getenv() 读取，严禁源码泄露凭据。",
		"SQLInjection":    "【指令5：参数化查询】严禁拼接 SQL 字符串，必须改用数据库驱动的占位符（? 或 $1）。",
	}

	// 2. 自动识别并提取相关的修复指导（ACM 风格的关键词搜索）
	var activeHints []string
	for category, hint := range allHints {
		if strings.Contains(combinedCheckers, category) {
			activeHints = append(activeHints, hint)
		}
	}

	// 3. 构建高度结构化的复合 Prompt
	prompt := fmt.Sprintf(
		"【任务】你是一个资深的 Go 语言专家。请针对以下代码片段，一并修复其中存在的【%d】个缺陷。\n"+
			"【待修复点清单】%s\n"+
			"【涉及核心变量】%s\n"+
			"【专项修复要求】\n%s\n"+
			"【原始代码片段】\n%s\n",
		len(activeHints),
		combinedCheckers,
		varName,
		strings.Join(activeHints, "\n"), // 把多个指令换行拼在一起
		codeSnippet,
	)

	// 4. 注入编译器 Stderr 反馈（自愈环）
	if contextErr != "" {
		prompt += fmt.Sprintf(
			"\n【重要纠错】你之前的尝试导致了编译报错，请务必根据此信息修正补丁：\n%s\n",
			contextErr)
	}

	// 5. 最终约束
	prompt += "\n【输出约束】\n" +
		"1. 仅返回修复后的纯 Go 代码片段。\n" +
		"2. 严禁任何文字解释，严禁包含 Markdown 标签（如 ```go）。\n" +
		"3. 必须确保修复后的补丁能完美覆盖并替换原始代码，且逻辑完整。"

	return callAI(prompt)
}

func callAI(prompt string) (string, error) {
	cfg := config.GlobalConfig.AI

	reqBody := AIRequest{
		Model: cfg.Model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}
	jsonData, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", cfg.APIURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// --- 【新增调试行】 ---
	// 如果你在这里看不到输出，说明网络请求根本没回来
	// fmt.Printf("DEBUG: API 原始返回: %s\n", string(body))

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API 报错码 %d: %s", resp.StatusCode, string(body))
	}

	var aiResp AIResponse
	if err := json.Unmarshal(body, &aiResp); err != nil {
		return "", fmt.Errorf("JSON 解析失败: %v", err)
	}

	if len(aiResp.Choices) == 0 {
		return "", fmt.Errorf("AI 没说话")
	}

	// 清洗代码：去掉可能的 markdown 标记
	result := aiResp.Choices[0].Message.Content
	result = strings.TrimPrefix(result, "```go")
	result = strings.TrimSuffix(result, "```")
	return strings.TrimSpace(result), nil
}
