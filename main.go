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

const (
	DEBUG = false

	toolCallFinishReason = "tool_calls"
)

type LLMClient struct {
	url          string
	model        string
	system       string
	summary      string
	messages     []Message
	contextLimit int
	contextUsed  int
	tools        map[string]Tool
}

func (c *LLMClient) getToolsList() []Tool {
	toolsList := make([]Tool, 0)
	for _, tool := range c.tools {
		toolsList = append(toolsList, tool)
	}
	return toolsList
}

type Tool interface {
	GetName() string
	GetDescription() string
	Exec(args string) string
}

type FunctionTool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
	execFn   func(string) string
}

func (f *FunctionTool) GetName() string {
	return f.Function.Name
}

func (f *FunctionTool) GetDescription() string {
	return f.Function.Description
}

func (f *FunctionTool) Exec(args string) string {
	return f.execFn(args)
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

type ToolParameters struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required"`
}

func NewClient(tools []Tool) *LLMClient {
	toolsMap := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		toolsMap[tool.GetName()] = tool
	}

	client := &LLMClient{
		url:          "http://localhost:1234/v1/chat/completions", // LM Studio API
		model:        "qwen/qwen3.5-35b-a3b",
		system:       "you are a senior software engineer. ",
		summary:      "",
		contextLimit: 200000,
		contextUsed:  0,
		tools:        toolsMap,
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
		toks, err := calculateTokens(msg.Content)
		if err != nil {
			return 0, err
		}
		total += toks
	}
	return total, nil
}

func calculateTokens(content string) (int, error) {
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
	System    Role = "system"
	User      Role = "user"
	Assistant Role = "assistant"
	ToolRole  Role = "tool"
)

type Message struct {
	Role         Role       `json:"role"`
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls"`
	FinishReason string     `json:"finish_reason"`
}

type ToolCall struct {
	Index    int              `json:"index"`
	Id       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature"`
	Stream      bool      `json:"stream"`
	Tools       []Tool    `json:"tools"`
	ToolChoice  string    `json:"tool_choice,omitempty"`
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

type ToolCallsMap map[string]string

type ToolCallArgs map[string]string

func (c *LLMClient) StreamSSE(messages []Message) (ToolCallsMap, error) {
	data := &Request{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.7,
		Stream:      true,
		Tools:       c.getToolsList(),
		ToolChoice:  "auto",
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}

	fmt.Println("json data: ", string(jsonData))
	// debugPrint("json data: %v \n", string(jsonData))

	resp, err := http.Post(c.url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	scanner := bufio.NewScanner(reader)

	totalResponse := ""

	toolCalls := make(ToolCallsMap, 0)
	lastFunctionName := ""

	for scanner.Scan() {
		line := scanner.Text()

		debugPrint("Line: %v \n", line)

		if after, ok := strings.CutPrefix(line, "data:"); ok {
			data := after
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
				return nil, fmt.Errorf("JSON Parsing error: data %v, error: %v", data, err)
			} else if len(response.Choices) > 0 {
				firstChoice := response.Choices[0]
				if firstChoice.FinishReason == toolCallFinishReason {
					return toolCalls, nil
				}
				if len(firstChoice.Delta.Content) > 0 {
					fmt.Print(firstChoice.Delta.Content)
					totalResponse += firstChoice.Delta.Content
				} else if len(firstChoice.Delta.ToolCalls) > 0 {

					for _, toolCall := range firstChoice.Delta.ToolCalls {
						if toolCall.Function.Name != "" {
							lastFunctionName = toolCall.Function.Name
						} else if toolCall.Function.Arguments != "" {
							toolCalls[lastFunctionName] = toolCall.Function.Arguments
						}
					}

				}
			} else {
				fmt.Println("Response data:  ", data)
			}
		} else {
			line = strings.TrimSpace(line)
			fmt.Print(line)
		}
	}

	if len(totalResponse) > 0 {
		c.AddMessage(Assistant, totalResponse)
	}

	return nil, err
}

func main() {
	Tools := []Tool{
		&ExecTool,
	}

	c := NewClient(Tools)
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

		// Allow user to exit gracefully by typing "quit" or "exit"
		if input == "quit" || input == "exit" {
			fmt.Println("\nExiting...")
			break
		}

		err = c.AddMessage(User, input)
		if err != nil {
			fmt.Printf("Error adding message: %v\n", err)
			continue
		}

		toolCalls, err := c.StreamSSE(c.messages)
		if err != nil {
			fmt.Println("ERROR", err)
			break
		}

		for len(toolCalls) > 0 {
			c.doToolCalls(toolCalls)

			toolCalls, err = c.StreamSSE(c.messages)
			if err != nil {
				fmt.Println("ERROR", err)
				break
			}
		}

		fmt.Printf("\nTokens used: %v%%, %v of %v\n\n", c.contextUsed*100/c.contextLimit, c.contextUsed, c.contextLimit)
	}
}

func (c *LLMClient) doToolCalls(toolCalls ToolCallsMap) []string {
	for toolName, toolCallArgs := range toolCalls {
		c.AddMessage(Assistant, fmt.Sprintf("Calling tool: %s with args: %s", toolName, toolCallArgs))
		result := c.doToolCall(toolName, toolCallArgs)
		c.AddMessage(ToolRole, result)
	}

	return nil
}

func (c *LLMClient) doToolCall(fname string, args string) string {
	return c.tools[fname].Exec(args)
}

func debugPrint(template string, args ...interface{}) {
	if DEBUG {
		fmt.Printf(template, args...)
	}
}
