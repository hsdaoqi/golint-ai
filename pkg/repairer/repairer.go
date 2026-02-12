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

// GetFix 向 AI 请求修复建议
func GetFix(varName, codeSnippet, contextErr string) (string, error) {
	// 构造 Prompt
	prompt := fmt.Sprintf(
		"你是一个 Go 语言专家。变量 '%s' 是 error 类型但未被检查。\n"+
			"代码片段：\n%s\n", varName, codeSnippet)

	// 如果有上次失败的编译错误，加入反馈
	if contextErr != "" {
		prompt += fmt.Sprintf("\n注意：上次修复导致了以下编译错误，请修正：\n%s", contextErr)
	}

	prompt += "\n请仅返回修复后的 Go 代码，不要任何解释，不要 Markdown 标签。"

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
