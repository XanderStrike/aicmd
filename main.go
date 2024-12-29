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

		// Create completion request with full message history
		resp, err := client.CreateChatCompletion(
			context.Background(),
			openai.ChatCompletionRequest{
				Model:    openai.GPT3Dot5Turbo,
				Messages: messages,
			},
		)

		if err != nil {
			fmt.Printf("Error generating command: %v\n", err)
			continue
		}

		command := strings.TrimSpace(resp.Choices[0].Message.Content)
		fmt.Printf("generated command: %s\n\n", command)

		// Add assistant's response to message history
		messages = append(messages, resp.Choices[0].Message)

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

					// Create completion request with error message
					resp, err := client.CreateChatCompletion(
						context.Background(),
						openai.ChatCompletionRequest{
							Model:    openai.GPT4o,
							Messages: messages,
						},
					)

					if err != nil {
						fmt.Printf("Error generating fix: %v\n", err)
						continue
					}

					// Display the AI's response
					fixedCommand := strings.TrimSpace(resp.Choices[0].Message.Content)
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
