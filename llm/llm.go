package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	provider string
	apiKey   string
	baseURL  string
	model    string
	http     *http.Client
}

type openAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature int       `json:"temperature"`
}

type ollamaRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

func New(provider, apiKey, baseURL, model string) *Client {
	switch provider {
	case "ollama":
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
	case "openai":
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		if model == "" {
			model = "gpt-4"
		}
	}
	if model == "" {
		model = "local-model"
	}
	return &Client{provider: provider, apiKey: apiKey, baseURL: baseURL, model: model, http: &http.Client{}}
}

func (c *Client) Chat(messages []Message) (string, error) {
	if c.provider == "ollama" {
		return c.ollamaChat(messages)
	}
	return c.openaiChat(messages)
}

func (c *Client) openaiChat(messages []Message) (string, error) {
	body, err := json.Marshal(openAIRequest{Model: c.model, Messages: messages, Temperature: 0})
	if err != nil {
		return "", fmt.Errorf("encode OpenAI request: %w", err)
	}
	req, err := http.NewRequest("POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read OpenAI response with status %d: %w", resp.StatusCode, err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("decode OpenAI response with status %d: %w", resp.StatusCode, err)
	}
	if result.Error.Message != "" {
		return "", fmt.Errorf("openai: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("OpenAI response status %d contained no choices and error %q", resp.StatusCode, result.Error.Message)
	}
	return result.Choices[0].Message.Content, nil
}

func (c *Client) ollamaChat(messages []Message) (string, error) {
	body, err := json.Marshal(ollamaRequest{Model: c.model, Messages: messages, Stream: false})
	if err != nil {
		return "", fmt.Errorf("encode Ollama request: %w", err)
	}
	req, err := http.NewRequest("POST", c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Error string `json:"error"`
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read Ollama response with status %d: %w", resp.StatusCode, err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("decode Ollama response with status %d: %w", resp.StatusCode, err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("ollama: %s", result.Error)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf(
			"Ollama response status %d contained message %q and error %q",
			resp.StatusCode, result.Message.Content, result.Error,
		)
	}
	if result.Message.Content == "" {
		return "", fmt.Errorf("Ollama response status %d contained an empty message", resp.StatusCode)
	}
	return result.Message.Content, nil
}
