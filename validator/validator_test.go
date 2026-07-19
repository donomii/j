package validator

import "testing"

func TestParseResultRequiresDefinedValidationSchema(t *testing.T) {
	result, err := parseResult(`{"valid":false,"feedback":"command exited with code 1"}`)
	if err != nil {
		t.Fatalf("parse valid validation response: %v", err)
	}
	if result.Valid || result.Feedback == "" {
		t.Fatalf("parsed result=%+v, expected invalid result with feedback", result)
	}
	invalidResponses := []string{
		"```json\n{\"valid\":true,\"feedback\":\"\"}\n```",
		`{"valid":false,"feedback":""}`,
		`{"feedback":"missing valid"}`,
		`{"valid":true}`,
		`{"valid":true,"feedback":"","extra":true}`,
		`{"valid":true,"feedback":""} trailing`,
	}
	for _, content := range invalidResponses {
		if _, err := parseResult(content); err == nil {
			t.Fatalf("invalid validation response was accepted: %q", content)
		}
	}
}
