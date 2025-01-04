package main

import (
	"context"
	openai "github.com/sashabaranov/go-openai"
)

type OpenAIClient struct {
	client *openai.Client
	model  string
}

func (c *OpenAIClient) GenerateCompletion(ctx context.Context, messages []openai.ChatCompletionMessage) (string, error) {
	model := c.model
	if model == "" {
		model = openai.GPT4o
	}
	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:    model,
			Messages: messages,
		},
	)
	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}
