package orchestrator

import "testing"

func TestParsePlanRequiresExactJSON(t *testing.T) {
	steps, err := parsePlan(`["inspect", "change", "verify"]`)
	if err != nil {
		t.Fatalf("parse valid plan: %v", err)
	}
	if len(steps) != 3 || steps[1] != "change" {
		t.Fatalf("parsed plan=%q, expected three ordered steps", steps)
	}
	invalidPlans := []string{
		"```json\n[\"inspect\"]\n```",
		`["inspect"] trailing`,
		`[]`,
		`["inspect", ""]`,
		`{"steps":["inspect"]}`,
	}
	for _, content := range invalidPlans {
		if _, err := parsePlan(content); err == nil {
			t.Fatalf("invalid plan was accepted: %q", content)
		}
	}
}

func TestParseStepInstructionUsesDefinedArguments(t *testing.T) {
	content := `{"thought":"run checks","tool":"executor","args":{"command":"go","args":["test","./..."],"cwd":"project"}}`
	instruction, err := parseStepInstruction(content)
	if err != nil {
		t.Fatalf("parse valid executor instruction: %v", err)
	}
	if instruction.Args.Command != "go" || len(instruction.Args.Arguments) != 2 {
		t.Fatalf("parsed instruction=%+v, expected executable and two arguments", instruction)
	}
	invalidInstructions := []string{
		`{"thought":"run","tool":"executor","args":{"command":"go","args":"test"}}`,
		`{"thought":"edit","tool":"editor","args":{"method":"applyPatch","path":"a","search":"","replace":"b"}}`,
		`{"thought":"run","tool":"executor","args":{"command":"go","args":[],"extra":true}}`,
		`{"thought":"run","tool":"unknown","args":{}}`,
	}
	for _, content := range invalidInstructions {
		if _, err := parseStepInstruction(content); err == nil {
			t.Fatalf("invalid instruction was accepted: %s", content)
		}
	}
}
