package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"encoding/json"
	"net/http"

	openai "github.com/sashabaranov/go-openai"
)

const prompt = `You are a command line assistant. Generate a single bash command
that accomplishes the user's request.  Only output the command itself, no
explanation or markdown formatting.  The command should be safe and should not
perform destructive operations without user confirmation.  Request: %s`

type OllamaRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaResponse struct {
	Model     string  `json:"model"`
	Created   int64   `json:"created_at"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
}

type Client interface {
	GenerateCompletion(ctx context.Context, messages []openai.ChatCompletionMessage) (string, error)
}

type OpenAIClient struct {
	client *openai.Client
}

func (c *OpenAIClient) GenerateCompletion(ctx context.Context, messages []openai.ChatCompletionMessage) (string, error) {
	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: messages,
		},
	)
	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("error decoding response: %v\nBody: %s", err, string(body))
	}

	if ollamaResp.Message.Content == "" {
		return "", fmt.Errorf("empty response from Ollama API: %s", string(body))
	}

	return ollamaResp.Message.Content, nil
}

func getClient() (Client, error) {
	if ollamaBase := os.Getenv("OLLAMA_API_BASE"); ollamaBase != "" {
		if model := os.Getenv("OLLAMA_MODEL"); model != "" {
			return &OllamaClient{
				baseURL: ollamaBase,
				model:   model,
			}, nil
		}
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("neither Ollama nor OpenAI configuration found")
	}
	return &OpenAIClient{client: openai.NewClient(apiKey)}, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: aicmd \"your command description\"")
		os.Exit(1)
	}

	// Initialize AI client
	client, err := getClient()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Keep track of conversation history
	messages := []openai.ChatCompletionMessage{}

	// Main interaction loop
	for {
		var userRequest string
		if len(messages) == 0 && len(os.Args) > 1 {
			// First request comes from command line args
			userRequest = strings.Join(os.Args[1:], " ")
		} else {
			// Subsequent requests come from stdin
			fmt.Print("\nEnter follow-up request (or 'exit' to quit): ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			userRequest = strings.TrimSpace(input)

			if userRequest == "exit" {
				break
			}
		}

		// Add user request to messages
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: fmt.Sprintf(prompt, userRequest),
		})

		// Generate completion with full message history
		completion, err := client.GenerateCompletion(context.Background(), messages)
		if err != nil {
			fmt.Printf("Error generating command: %v\n", err)
			continue
		}

		command := strings.TrimSpace(completion)
		fmt.Printf("generated command: %s\n\n", command)

		// Add assistant's response to message history
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: command,
		})

		// Ask for confirmation with follow-up option
		fmt.Print("run it now? [Y/n/f for fix]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))

		if response == "f" {
			continue
		} else if response == "" || response == "y" || response == "yes" {
			// Execute the command
			cmd := exec.Command("bash", "-c", command)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
			cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
			err := cmd.Run()

			// Check the exit code of the command
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode := exitError.ExitCode()
				if exitCode != 0 {
					fmt.Printf("Command exited with code %d\n", exitCode)

					// Send stdout and stderr to AI for explanation and fix
					errorMessage := fmt.Sprintf("stdout: %s\nstderr: %s\n", stdout.String(), stderr.String())
					messages = append(messages, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleUser,
						Content: fmt.Sprintf("The command failed with the following output:\n%s\nPlease explain the error and provide a fixed command.", errorMessage),
					})

					// Generate completion with error message
					completion, err := client.GenerateCompletion(context.Background(), messages)
					if err != nil {
						fmt.Printf("Error generating fix: %v\n", err)
						continue
					}

					// Display the AI's response
					fixedCommand := strings.TrimSpace(completion)
					fmt.Printf("AI suggested fix: %s\n\n", fixedCommand)
					continue
				}
			} else if err != nil {
				fmt.Printf("Error executing command: %v\n", err)
				continue
			}
			// Exit if the command runs successfully
			return
		}
	}
}
