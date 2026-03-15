package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

var ExecTool = FunctionTool{
	Type: "function",
	execFn: func(arg string) string {
		fmt.Println(arg)
		tca := make(ToolCallArgs)
		err := json.Unmarshal([]byte(arg), &tca)
		if err != nil {
			return fmt.Sprintf("Error parsing tool call arguments: %v", err)
		}
		cmd := tca["command"]
		output, err := executeCommand(cmd)
		if err != nil {
			return fmt.Sprintf("Error executing command: %v", err)
		}
		return output
	},
	Function: ToolFunction{
		Name:        "exec_command",
		Description: "Execute a shell command and return the output",
		Parameters: ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The shell command to execute",
				},
			},
			Required: []string{"command"},
		},
	},
}

func executeCommand(cmd string) (string, error) {
	result, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	return string(result), err
}
