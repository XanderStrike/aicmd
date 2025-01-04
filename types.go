package main

type Provider string

const (
	ProviderAnthropicDefault = "claude-3-5-sonnet-latest"
	ProviderOpenAIDefault    = "gpt-4"
	ProviderOllamaDefault    = "llama2"

	ProviderAnthropicKey = "ANTHROPIC_API_KEY"
	ProviderOpenAIKey    = "OPENAI_API_KEY"
	ProviderOllamaBase   = "OLLAMA_API_BASE"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
