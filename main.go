package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/tiktoken-go/tokenizer"
)

type LLMClient struct {
	url          string
	model        string
	system       string
	summary      string
	messages     []Message
	contextLimit int
	contextUsed  int
}

func NewClient() *LLMClient {
	client := &LLMClient{
		url:          "http://localhost:1234/v1/chat/completions", // LM Studio API
		model:        "qwen/qwen3.5-35b-a3b",
		system:       "you are a senior software engineer",
		summary:      "",
		contextLimit: 200000,
		contextUsed:  0,
	}

	client.messages = []Message{
		{Role: System, Content: client.system},
	}

	return client
}

func (c *LLMClient) AddMessage(role Role, content string) error {
	var err error

	c.messages = append(c.messages, Message{Role: role, Content: content})
	c.contextUsed, err = c.calculateContextUsed()
	if err != nil {
		fmt.Printf("Error calculating context used: %v\n", err)
		return err
	}
	return nil
}

func (c *LLMClient) calculateContextUsed() (int, error) {
	total := 0
	for _, msg := range c.messages {
		toks, err := сalculateTokens(msg.Content)
		if err != nil {
			return 0, err
		}
		total += toks
	}
	return total, nil
}

func сalculateTokens(content string) (int, error) {
	// This is a very naive token calculation. In a real implementation, you would want to use a proper tokenizer for the model you're using.
	toker, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		return 0, err
	}

	count, err := toker.Count(content)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (c *LLMClient) Compact() {
	summary := c.Summarize()
	c.system += "\n\nSummary of conversation so far:\n" + summary + "\n\nContinue the conversation based on this summary. Here is your system prompt:\n" + c.system
	c.messages = []Message{
		{Role: System, Content: c.system},
	}
}

func (c *LLMClient) Summarize() string {
	summarizationPrompt := []Message{
		{Role: System, Content: "You are a helpful assistant that summarizes conversations."},
	}

	summarizationPrompt = append(summarizationPrompt, c.messages...)
	summarizationPrompt = append(summarizationPrompt, Message{Role: User, Content: "Please summarize the conversation so far in a concise manner."})

	data := &Request{
		Model:       c.model,
		Messages:    summarizationPrompt,
		Temperature: 0.7,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Println("Error marshaling summarization request:", err)
		return ""
	}

	resp, err := http.Post(c.url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error making summarization request:", err)
		return ""
	}
	defer resp.Body.Close()

	response := &Response{}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading summarization response:", err)
		return ""
	}

	err = json.Unmarshal([]byte(body), response)
	if err != nil {
		fmt.Println("Error unmarshaling summarization response:", err)
		return ""
	}

	return response.Choices[0].Message.Content
}

type Role string

const (
	System Role = "system"
	User   Role = "user"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature"`
	Stream      bool      `json:"stream"`
}

type Response struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage"`
	Stats             struct{} `json:"stats"`
	SystemFingerPrint string   `json:"system_fingerprint"`
}

type Choice struct {
	Index        int       `json:"index"`
	Message      Message   `json:"message"`
	Logprobs     *struct{} `json:"logprobs"` // Optional, can be null
	FinishReason string    `json:"finish_reason"`
	Delta        Message   `json:"delta"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (c *LLMClient) StreamSSE(messages []Message) error {
	data := &Request{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.7,
		Stream:      true,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := http.Post(c.url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	scanner := bufio.NewScanner(reader)

	totalResponse := ""

	for scanner.Scan() {
		line := scanner.Text()
		// fmt.Println("Line: ", line)

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			if len(data) == 0 {
				continue
			}

			if data == "[DONE]" {
				fmt.Println()
				break
			}

			// fmt.Println("Data: ", data)

			var response Response

			if err := json.Unmarshal([]byte(data), &response); err != nil {
				return fmt.Errorf("JSON Parsing error: data %v, error: %v", data, err)
			} else if len(response.Choices) > 0 {
				fmt.Print(response.Choices[0].Delta.Content)
				totalResponse += response.Choices[0].Delta.Content
			} else {
				fmt.Println("Response data:  ", data)
			}
		} else {
			// fmt.Println(line)
		}
	}

	if len(totalResponse) > 0 {
		c.AddMessage(Role("assistant"), totalResponse)
	}

	return nil
}

func main() {
	c := NewClient()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(">>>: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nEOF received. Exiting.")
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)

		err = c.AddMessage(User, input)
		if err != nil {
			fmt.Printf("Error adding message: %v\n", err)
			continue
		}

		err = c.StreamSSE(c.messages)
		if err != nil {
			fmt.Println(err)
			break
		}

		fmt.Printf("Tokens used: %v %%, %v of %v\n\n", c.contextUsed*100/c.contextLimit, c.contextUsed, c.contextLimit)
	}
}
