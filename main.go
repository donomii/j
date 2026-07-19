package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/donomii/j/llm"
	"github.com/donomii/j/orchestrator"
	"github.com/donomii/j/runner"
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

func ask(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return scanner.Text()
}

func main() {
	loadDotenv()

	provider := os.Getenv("LLM_PROVIDER")
	if provider == "" {
		provider = "local"
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	modelName := os.Getenv("LLM_MODEL_NAME")
	githubToken := os.Getenv("GITHUB_TOKEN")
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

	if provider == "openai" && apiKey == "" {
		fmt.Fprintln(os.Stderr, "Please set OPENAI_API_KEY in .env")
		os.Exit(1)
	}

	if provider == "local" {
		modelURL := os.Getenv("MODEL_URL")
		if modelURL == "" {
			modelURL = runner.DefaultModelURL
		}
		serverBin, err := runner.EnsureServer()
		if err != nil {
			log.Fatalf("ensure llama-server: %v", err)
		}
		modelPath, err := runner.EnsureModel(modelURL)
		if err != nil {
			log.Fatalf("ensure model: %v", err)
		}
		serverURL, stop, err := runner.Start(serverBin, modelPath)
		if err != nil {
			log.Fatalf("start server: %v", err)
		}
		defer stop()
		baseURL = serverURL
	}

	llmClient := llm.New(provider, apiKey, baseURL, modelName)
	orch := orchestrator.New(llmClient, githubToken, workspaceRoot, useDocker, executorImage)

	prompt := ask("What task should I perform? ")

	fmt.Printf("Generating plan using %s...\n", provider)
	plan, err := orch.GeneratePlan(prompt)
	if err != nil {
		log.Fatalf("generate plan: %v", err)
	}

	fmt.Println("Proposed Plan:")
	for _, step := range plan {
		fmt.Printf("%d. %s\n", step.ID, step.Description)
	}

	approval := ask("Do you approve this plan? (yes/no) ")
	if strings.ToLower(strings.TrimSpace(approval)) != "yes" {
		fmt.Println("Plan rejected. Task aborted.")
		return
	}

	fmt.Println("Executing plan...")
	if err := orch.ExecutePlan(func(step types.PlanStep) {
		fmt.Printf("Step %d completed.\n", step.ID)
	}); err != nil {
		log.Fatalf("execute plan: %v", err)
	}
	fmt.Println("Task completed successfully.")
}
