package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var WeatherTool = FunctionTool{
	Type: "function",
	execFn: func(arg string) string {
		fmt.Println(arg)
		tca := make(ToolCallArgs)
		err := json.Unmarshal([]byte(arg), &tca)
		if err != nil {
			return fmt.Sprintf("Error parsing tool call arguments: %v", err)
		}
		const apiKey = "c97680a34e494868a3d203825261503"
		url := "http://api.weatherapi.com/v1/current.json?key=" + apiKey + "&q=" + tca["location"] + "&aqi=no"
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Sprintf("Error making weather API request: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Sprintf("Error reading weather API response: %v", err)
		}

		var weatherResponse struct {
			Current struct {
				TempC     float64 `json:"temp_c"`
				TempF     float64 `json:"temp_f"`
				Condition struct {
					Text string `json:"text"`
				} `json:"condition"`
			} `json:"current"`
		}

		err = json.Unmarshal(body, &weatherResponse)
		if err != nil {
			return fmt.Sprintf("Error parsing weather API response: %v", err)
		}

		if tca["unit"] == "celsius" {
			return fmt.Sprintf("The current weather in %s is %.1f °C with %s.", tca["location"], weatherResponse.Current.TempC, weatherResponse.Current.Condition.Text)
		} else {
			return fmt.Sprintf("The current weather in %s is %.1f °F with %s.", tca["location"], weatherResponse.Current.TempF, weatherResponse.Current.Condition.Text)
		}
	},
	Function: ToolFunction{
		Name:        "get_current_weather",
		Description: "Get the current weather in a given location",
		Parameters: ToolParameters{
			Type: "object",
			Properties: map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The city and state, e.g. San Francisco, CA",
				},
				"unit": map[string]interface{}{
					"type": "string",
					"enum": []string{"celsius", "fahrenheit"},
				},
			},
			Required: []string{"location", "unit"},
		},
	},
}
