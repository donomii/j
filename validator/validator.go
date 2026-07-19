package validator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/donomii/j/llm"
	"github.com/donomii/j/types"
)

type Validator struct {
	llm *llm.Client
}

type Result struct {
	Valid    bool   `json:"valid"`
	Feedback string `json:"feedback"`
}

func New(llmClient *llm.Client) *Validator {
	return &Validator{llm: llmClient}
}

func (v *Validator) Validate(step string, output types.StepResult) (Result, error) {
	const sysPrompt = "You are a quality assurance engineer. Review the output of a task step. " +
		"Determine if the output is correct and moves towards the goal. " +
		`Return JSON { "valid": boolean, "feedback": "string" }`

	outputJSON, err := json.Marshal(output)
	if err != nil {
		return Result{}, fmt.Errorf("encode step output for validation: %w", err)
	}
	msgs := []llm.Message{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: fmt.Sprintf("Step: %s\nOutput: %s", step, outputJSON)},
	}

	content, err := v.llm.Chat(msgs)
	if err != nil {
		return Result{}, err
	}

	result, err := parseResult(content)
	if err != nil {
		return Result{}, fmt.Errorf("parse validation response: %w", err)
	}
	return result, nil
}

func parseResult(content string) (Result, error) {
	type validationResponse struct {
		Valid    *bool   `json:"valid"`
		Feedback *string `json:"feedback"`
	}
	var response validationResponse
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return Result{}, fmt.Errorf("expected strict validation JSON object: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Result{}, fmt.Errorf("expected one validation JSON object, received trailing value %s", trailing)
		}
		return Result{}, fmt.Errorf("check trailing validation JSON: %w", err)
	}
	if response.Valid == nil {
		return Result{}, fmt.Errorf("validation response is missing required boolean field valid")
	}
	if response.Feedback == nil {
		return Result{}, fmt.Errorf("validation response is missing required string field feedback")
	}
	if !*response.Valid && *response.Feedback == "" {
		return Result{}, fmt.Errorf("invalid validation response requires non-empty feedback")
	}
	return Result{Valid: *response.Valid, Feedback: *response.Feedback}, nil
}
