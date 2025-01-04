package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"bytes"
	"io/ioutil"

	openai "github.com/sashabaranov/go-openai"
)

type AnthropicClient struct {
	apiKey string
	model  string
}

type AnthropicRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}


type AnthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

func (c *AnthropicClient) GenerateCompletion(ctx context.Context, messages []openai.ChatCompletionMessage) (string, error) {
	model := c.model
	if model == "" {
		model = "claude-2"
	}

	// Convert OpenAI messages to Anthropic format
	anthropicMessages := make([]Message, len(messages))
	for i, msg := range messages {
		role := msg.Role
		if role == "assistant" {
			role = "assistant"
		} else {
			role = "user"
		}
		anthropicMessages[i] = Message{
			Role:    role,
			Content: msg.Content,
		}
	}

	reqBody := AnthropicRequest{
		Model:     model,
		Messages:  anthropicMessages,
		MaxTokens: 1024,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	if len(anthropicResp.Content) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return anthropicResp.Content[0].Text, nil
}
