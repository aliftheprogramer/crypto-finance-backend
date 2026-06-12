package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type DeepSeekProvider struct {
	apiKey string
	http   *http.Client
}

func NewDeepSeekProvider(apiKey string) *DeepSeekProvider {
	return &DeepSeekProvider{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

type deepseekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepseekRequest struct {
	Model       string            `json:"model"`
	Messages    []deepseekMessage `json:"messages"`
	Temperature float64           `json:"temperature"`
	MaxTokens   int               `json:"max_tokens"`
}

type deepseekUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type deepseekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *deepseekUsage `json:"usage"`
}

func (d *DeepSeekProvider) Analyze(prompt string) (content string, promptTokens, completionTokens int, err error) {
	return d.Chat("Kamu adalah analis kripto profesional. Selalu berikan analisa dalam bahasa Indonesia. Output dalam format JSON.", prompt, 0.3, 1000)
}

func (d *DeepSeekProvider) Chat(systemPrompt, userPrompt string, temperature float64, maxTokens int) (content string, promptTokens, completionTokens int, err error) {
	body := deepseekRequest{
		Model: "deepseek-chat",
		Messages: []deepseekMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", 0, 0, fmt.Errorf("deepseek marshal: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.deepseek.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", 0, 0, fmt.Errorf("deepseek create req: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.http.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("deepseek request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, 0, fmt.Errorf("deepseek read: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, 0, fmt.Errorf("deepseek API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result deepseekResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", 0, 0, fmt.Errorf("deepseek parse: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("deepseek: no choices returned")
	}

	pt, ct := 0, 0
	if result.Usage != nil {
		pt = result.Usage.PromptTokens
		ct = result.Usage.CompletionTokens
	}

	return result.Choices[0].Message.Content, pt, ct, nil
}
