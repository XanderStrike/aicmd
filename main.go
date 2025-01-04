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
	"flag"

	"github.com/fatih/color"

	"encoding/json"

	openai "github.com/sashabaranov/go-openai"
)

const prompt = `You are a command line assistant. Generate a single bash command
that accomplishes the user's request. IMPORTANT: Your response must be a JSON object that can be directly parsed, no markdown (seriously NO code blocks) or styling at all. Escape quotes in the json so that it can be parsed correctly.
The json must have exactly two fields: "command" containing the raw command text, and "description"
containing a detailed explanation of how the command works, breaking down each component
and flag being used. Example:
{"command": "ls -la", "description": "Uses 'ls' (list) command with '-l' flag for long format showing permissions and sizes, and '-a' flag to show hidden files starting with dot"}
The command should be safe and should not perform destructive operations without
user confirmation. Request: %s`

type CommandResponse struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

type Client interface {
	GenerateCompletion(ctx context.Context, messages []openai.ChatCompletionMessage) (string, error)
}

type OpenAIClient struct {
	client *openai.Client
}

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

func getClient(forceOpenAI bool, forceOllama bool, model string) (Client, error) {
	if forceOpenAI && forceOllama {
		return nil, fmt.Errorf("cannot force both OpenAI and Ollama at the same time")
	}

	if forceOpenAI {
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key not found in environment")
		}
		return &OpenAIClient{
			client: openai.NewClient(apiKey),
			model:  model,
		}, nil
	}

	if forceOllama {
		ollamaBase := os.Getenv("OLLAMA_API_BASE")
		if ollamaBase == "" {
			return nil, fmt.Errorf("OLLAMA_API_BASE not found in environment")
		}
		ollamaModel := model
		if ollamaModel == "" {
			ollamaModel = os.Getenv("OLLAMA_MODEL")
			if ollamaModel == "" {
				return nil, fmt.Errorf("no model specified and OLLAMA_MODEL not found in environment")
			}
		}
		return &OllamaClient{
			baseURL: ollamaBase,
			model:   ollamaModel,
		}, nil
	}

	// Default behavior: try Ollama first, then OpenAI
	if ollamaBase := os.Getenv("OLLAMA_API_BASE"); ollamaBase != "" {
		ollamaModel := model
		if ollamaModel == "" {
			ollamaModel = os.Getenv("OLLAMA_MODEL")
			if ollamaModel != "" {
				return &OllamaClient{
					baseURL: ollamaBase,
					model:   ollamaModel,
				}, nil
			}
		} else {
			return &OllamaClient{
				baseURL: ollamaBase,
				model:   ollamaModel,
			}, nil
		}
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("neither Ollama nor OpenAI configuration found")
	}
	return &OpenAIClient{
		client: openai.NewClient(apiKey),
		model:  model,
	}, nil
}

func main() {
	forceOpenAI := flag.Bool("openai", false, "Force using OpenAI")
	forceOllama := flag.Bool("ollama", false, "Force using Ollama")
	model := flag.String("model", "", "Specify model to use (defaults to GPT-4 for OpenAI)")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("Usage: aicmd [--openai|--ollama] [--model MODEL] \"your command description\"")
		os.Exit(1)
	}

	// Initialize AI client
	client, err := getClient(*forceOpenAI, *forceOllama, *model)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Announce which service is being used
	switch c := client.(type) {
	case *OllamaClient:
		fmt.Printf("Using Ollama with model: %s\n", c.model)
	case *OpenAIClient:
		modelName := openai.GPT4o
		if c.model != "" {
			modelName = c.model
		}
		fmt.Printf("Using OpenAI with model: %s\n", modelName)
	}

	// Keep track of conversation history
	messages := []openai.ChatCompletionMessage{}

	// Main interaction loop
	for {
		var userRequest string
		if len(messages) == 0 && len(os.Args) > 1 {
			// First request comes from command line args
			userRequest = strings.Join(flag.Args(), " ")
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

		// Parse the JSON response
		var cmdResponse CommandResponse
		if err := json.Unmarshal([]byte(completion), &cmdResponse); err != nil {
			fmt.Printf("Error parsing response: %v\nResponse: %s\n", err, completion)
			continue
		}

		command := strings.TrimSpace(cmdResponse.Command)

		// Print with colors
		color.Blue("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		color.Green("▶ Command:")
		color.Yellow("  %s", command)
		color.Green("\n▶ Description:")
		color.White("  %s", cmdResponse.Description)
		color.Blue("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

		// Add assistant's response to message history
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: command,
		})

		// Ask for confirmation with follow-up option
		color.HiCyan("Run it now? [Y/n/f for fix]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))

		if response == "f" {
			continue
		} else if response == "" || response == "y" || response == "yes" {
		executeCommand:
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

					// Parse the JSON response for the fix
					var fixResponse CommandResponse
					if err := json.Unmarshal([]byte(completion), &fixResponse); err != nil {
						fmt.Printf("Error parsing fix response: %v\n", err)
						continue
					}

					// Display the fix with colors
					color.Blue("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
					color.Red("⚠ Previous command failed. Here's the suggested fix:")
					color.Green("▶ Fixed Command:")
					color.Yellow("  %s", fixResponse.Command)
					color.Green("\n▶ Explanation:")
					color.White("  %s", fixResponse.Description)
					color.Blue("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

					// Ask to run the fixed command
					color.HiCyan("Run the fixed command? [Y/n]: ")
					fixUserResponse, _ := reader.ReadString('\n')
					fixUserResponse = strings.ToLower(strings.TrimSpace(fixUserResponse))

					if fixUserResponse == "" || fixUserResponse == "y" || fixUserResponse == "yes" {
						command = strings.TrimSpace(fixResponse.Command)
						goto executeCommand
					}
					continue
				}
			} else if err != nil {
				fmt.Printf("Error executing command: %v\n", err)
				continue
			}

			// Prompt user about adding output to context
			color.HiCyan("\nAdd command output to conversation context? [y/N]: ")
			contextResponse, _ := reader.ReadString('\n')
			contextResponse = strings.ToLower(strings.TrimSpace(contextResponse))

			if contextResponse == "y" || contextResponse == "yes" {
				outputContext := fmt.Sprintf("Command output:\n%s", stdout.String())
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: outputContext,
				})
				color.Green("Output added to conversation context")
			}

			// Continue to next iteration after successful command
			continue
		}
	}
}
