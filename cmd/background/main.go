package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/donomii/j/github"
	"github.com/donomii/j/llm"
	"github.com/donomii/j/orchestrator"
	"github.com/donomii/j/types"
	"github.com/donomii/j/workspace"
)

func loadDotenv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		os.Setenv(strings.TrimSpace(k), strings.TrimSpace(v))
	}
}

func main() {
	loadDotenv()

	githubToken := os.Getenv("GITHUB_TOKEN")
	owner := os.Getenv("REPO_OWNER")
	repo := os.Getenv("REPO_NAME")
	if githubToken == "" || owner == "" || repo == "" {
		log.Fatal("GITHUB_TOKEN, REPO_OWNER, and REPO_NAME must be set")
	}
	workspacePath := os.Getenv("J_WORKSPACE")
	if workspacePath == "" {
		var err error
		workspacePath, err = os.Getwd()
		if err != nil {
			log.Fatalf("resolve default workspace: %v", err)
		}
	}
	workspaceRoot, err := workspace.NewRoot(workspacePath)
	if err != nil {
		log.Fatalf("approve workspace %q: %v", workspacePath, err)
	}
	useDocker := strings.ToLower(os.Getenv("USE_DOCKER")) != "false"
	executorImage := os.Getenv("J_EXECUTOR_IMAGE")
	if executorImage == "" {
		executorImage = "node:20-slim"
	}

	provider := os.Getenv("LLM_PROVIDER")
	if provider == "" {
		provider = "openai"
	}
	llmClient := llm.New(provider, os.Getenv("OPENAI_API_KEY"), os.Getenv("OLLAMA_BASE_URL"), os.Getenv("LLM_MODEL_NAME"))
	gh := github.New(githubToken, workspaceRoot)

	fmt.Printf("Polling issues for %s/%s...\n", owner, repo)
	for {
		issues, err := gh.ListIssues(owner, repo, "agent-task")
		if err != nil {
			log.Printf("Error listing issues: %v", err)
		} else {
			for _, issue := range issues {
				if err := handleIssue(gh, llmClient, githubToken, workspaceRoot, useDocker, executorImage, owner, repo, issue); err != nil {
					log.Printf("Issue #%d failed: %v", issue.Number, err)
				}
			}
		}
		time.Sleep(60 * time.Second)
	}
}

func handleIssue(
	gh *github.Service,
	llmClient *llm.Client,
	token string,
	workspaceRoot types.WorkspaceRoot,
	useDocker bool,
	executorImage, owner, repo string,
	issue github.Issue,
) error {
	fmt.Printf("Starting task from issue #%d: %s\n", issue.Number, issue.Title)

	if err := gh.RemoveLabel(owner, repo, issue.Number, "agent-task"); err != nil {
		return fmt.Errorf("remove agent-task label from issue #%d: %w", issue.Number, err)
	}
	if err := gh.IssueComment(owner, repo, issue.Number, "I've picked up this task! I'm starting the planning phase now."); err != nil {
		return fmt.Errorf("announce start on issue #%d: %w", issue.Number, err)
	}

	orch := orchestrator.New(llmClient, token, workspaceRoot, useDocker, executorImage)
	prompt := issue.Body
	if prompt == "" {
		prompt = issue.Title
	}

	plan, err := orch.GeneratePlan(prompt)
	if err != nil {
		commentErr := gh.IssueComment(owner, repo, issue.Number, fmt.Sprintf("I encountered an error while planning: %v", err))
		if commentErr != nil {
			return fmt.Errorf("plan issue #%d: %v; report planning failure: %w", issue.Number, err, commentErr)
		}
		return fmt.Errorf("plan issue #%d: %w", issue.Number, err)
	}

	lines := make([]string, len(plan))
	for i, step := range plan {
		lines[i] = fmt.Sprintf("%d. %s", step.ID, step.Description)
	}
	planComment := fmt.Sprintf("Here is my proposed plan:\n\n%s\n\nI will now begin execution.", strings.Join(lines, "\n"))
	if err := gh.IssueComment(owner, repo, issue.Number, planComment); err != nil {
		return fmt.Errorf("publish plan for issue #%d: %w", issue.Number, err)
	}

	if err := orch.ExecutePlan(nil); err != nil {
		commentErr := gh.IssueComment(owner, repo, issue.Number, fmt.Sprintf("I encountered an error while performing the task: %v", err))
		if commentErr != nil {
			return fmt.Errorf("execute issue #%d: %v; report execution failure: %w", issue.Number, err, commentErr)
		}
		return fmt.Errorf("execute issue #%d: %w", issue.Number, err)
	}

	if err := gh.IssueComment(owner, repo, issue.Number, "I've completed the task! Please review the changes in the repository."); err != nil {
		return fmt.Errorf("announce completion on issue #%d: %w", issue.Number, err)
	}
	if err := gh.CloseIssue(owner, repo, issue.Number); err != nil {
		return fmt.Errorf("close completed issue #%d: %w", issue.Number, err)
	}
	return nil
}
