// Package types contains common type definitions for the LLM client application.
package types

// Role represents the role of a message in a conversation.
type Role string

const (
	System    Role = "system"
	User      Role = "user"
	Assistant Role = "assistant"
	ToolRole  Role = "tool"
)

// Message represents a message in the conversation.
type Message struct {
	Role         Role       `json:"role"`
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls"`
	FinishReason string     `json:"finish_reason"`
}

// ToolCall represents a tool call made by an assistant.
type ToolCall struct {
	Index    int              `json:"index"`
	Id       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction represents the function called in a tool call.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Request represents an API request to the LLM.
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature"`
	Stream      bool      `json:"stream"`
	Tools       []Tool    `json:"tools"`
	ToolChoice  string    `json:"tool_choice,omitempty"`
}

// Response represents an API response from the LLM.
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

// Choice represents a choice in the response.
type Choice struct {
	Index        int       `json:"index"`
	Message      Message   `json:"message"`
	Logprobs     *struct{} `json:"logprobs"`
	FinishReason string    `json:"finish_reason"`
	Delta        Message   `json:"delta"`
}

// Usage represents token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Tool is the interface for all tools.
type Tool interface {
	GetName() string
	GetDescription() string
	Exec(args string) string
}

// FunctionTool implements the Tool interface.
type FunctionTool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
	execFn   func(string) string
}

// ToolFunction represents a function tool definition.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

// ToolParameters defines the parameters for a function.
type ToolParameters struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required"`
}

// ToolCallsMap maps tool names to their arguments.
type ToolCallsMap map[string]string

// ToolCallArgs represents arguments for a tool call.
type ToolCallArgs map[string]string

// LLMClient wraps the LLM API client.
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

// Tokenizer is a simple interface for token counting.
type Tokenizer interface {
	Count(content string) (int, error)
}
