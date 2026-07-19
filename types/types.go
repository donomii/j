package types

type WorkspaceRoot string

type PlanStep struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type ExecutionResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

type PullRequestSummary struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"htmlUrl"`
}

type StepResult struct {
	Summary     string              `json:"summary"`
	Output      string              `json:"output,omitempty"`
	Execution   *ExecutionResult    `json:"execution,omitempty"`
	PullRequest *PullRequestSummary `json:"pullRequest,omitempty"`
}
