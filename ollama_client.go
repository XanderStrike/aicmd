package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type OllamaRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaResponse struct {
	Model   string  `json:"model"`
	Created string  `json:"created_at"`
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

type OllamaClient struct {
	baseURL string
	model   string
}

func (c *OllamaClient) GenerateCompletion(ctx context.Context, messages []openai.ChatCompletionMessage) (string, error) {
	// Convert OpenAI messages to Ollama format
	ollamaMessages := make([]Message, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	reqBody := OllamaRequest{
		Model:    c.model,
		Messages: ollamaMessages,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Use a scanner to read the streaming response line by line
	var fullContent strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var response OllamaResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			return "", fmt.Errorf("error decoding response line: %v\nLine: %s", err, line)
		}

		// Accumulate the content
		if response.Message.Content != "" {
			fullContent.WriteString(response.Message.Content)
		}

		// If we get a done message, we can stop
		if response.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading response stream: %v", err)
	}

	result := fullContent.String()
	if result == "" {
		return "", fmt.Errorf("no valid response received from Ollama API")
	}

	return result, nil
}
