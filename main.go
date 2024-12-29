package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const prompt = `You are a command line assistant. Generate a single bash command
that accomplishes the user's request.  Only output the command itself, no
explanation or markdown formatting.  The command should be safe and should not
perform destructive operations without user confirmation.  Request: %s`

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: aicmd \"your command description\"")
		os.Exit(1)
	}

	// Get OpenAI API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Error: OPENAI_API_KEY environment variable not set")
		os.Exit(1)
	}

	// Initialize OpenAI client
	client := openai.NewClient(apiKey)

	// Create completion request
	userRequest := strings.Join(os.Args[1:], " ")
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf(prompt, userRequest),
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("Error generating command: %v\n", err)
		os.Exit(1)
	}

	command := strings.TrimSpace(resp.Choices[0].Message.Content)
	fmt.Printf("generated command: %s\n\n", command)

	// Ask for confirmation
	fmt.Print("run it now? [Y/n]: ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	if response == "" || response == "y" || response == "yes" {
		// Execute the command
		cmd := exec.Command("bash", "-c", command)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error executing command: %v\n", err)
			os.Exit(1)
		}
	}
}
