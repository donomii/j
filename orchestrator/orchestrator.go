package orchestrator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/donomii/j/editor"
	"github.com/donomii/j/github"
	"github.com/donomii/j/llm"
	"github.com/donomii/j/sandbox"
	"github.com/donomii/j/search"
	"github.com/donomii/j/tester"
	"github.com/donomii/j/types"
	"github.com/donomii/j/validator"
)

type Orchestrator struct {
	llm       *llm.Client
	github    *github.Service
	executor  *sandbox.SandboxExecutor
	editor    *editor.CodeEditor
	tester    *tester.PlaywrightTester
	search    *search.WebSearchTool
	validator *validator.Validator
	plan      []types.PlanStep
	history   []llm.Message
}

func New(llmClient *llm.Client, githubToken string, workspaceRoot types.WorkspaceRoot, useDocker bool, executorImage string) *Orchestrator {
	return &Orchestrator{
		llm:       llmClient,
		github:    github.New(githubToken, workspaceRoot),
		executor:  sandbox.New(workspaceRoot, useDocker, executorImage),
		editor:    editor.New(workspaceRoot),
		tester:    tester.New(workspaceRoot),
		search:    search.New(),
		validator: validator.New(llmClient),
	}
}

func parsePlan(content string) ([]string, error) {
	var steps []string
	if err := decodeStrict(content, func(decoder *json.Decoder) error { return decoder.Decode(&steps) }); err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("plan must contain at least one step")
	}
	for index, step := range steps {
		if strings.TrimSpace(step) == "" {
			return nil, fmt.Errorf("plan step %d is empty", index+1)
		}
	}
	return steps, nil
}

func parseStepInstruction(content string) (stepInstruction, error) {
	var response struct {
		Thought string          `json:"thought"`
		Tool    string          `json:"tool"`
		Args    json.RawMessage `json:"args"`
	}
	if err := decodeStrict(content, func(decoder *json.Decoder) error { return decoder.Decode(&response) }); err != nil {
		return stepInstruction{}, err
	}
	if strings.TrimSpace(response.Thought) == "" {
		return stepInstruction{}, fmt.Errorf("step instruction thought is empty")
	}
	args, err := parseStepArguments(response.Tool, response.Args)
	if err != nil {
		return stepInstruction{}, err
	}
	instruction := stepInstruction{Thought: response.Thought, Tool: response.Tool, Args: args}
	if err := validateInstruction(instruction); err != nil {
		return stepInstruction{}, err
	}
	return instruction, nil
}

func parseStepArguments(tool string, raw json.RawMessage) (stepArguments, error) {
	switch tool {
	case "editor":
		method, err := parseMethod(raw, "editor")
		if err != nil {
			return stepArguments{}, err
		}
		switch method {
		case "writeFile":
			var value struct {
				Method  string `json:"method"`
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := decodeToolArguments(raw, "editor writeFile", func(decoder *json.Decoder) error { return decoder.Decode(&value) }); err != nil {
				return stepArguments{}, err
			}
			return stepArguments{Method: value.Method, Path: value.Path, Content: value.Content}, nil
		case "applyPatch":
			var value struct {
				Method  string `json:"method"`
				Path    string `json:"path"`
				Search  string `json:"search"`
				Replace string `json:"replace"`
			}
			err := decodeToolArguments(raw, "editor applyPatch", func(decoder *json.Decoder) error {
				return decoder.Decode(&value)
			})
			if err != nil {
				return stepArguments{}, err
			}
			return stepArguments{Method: value.Method, Path: value.Path, Search: value.Search, Replace: value.Replace}, nil
		case "readFile":
			var value struct {
				Method string `json:"method"`
				Path   string `json:"path"`
			}
			if err := decodeToolArguments(raw, "editor readFile", func(decoder *json.Decoder) error { return decoder.Decode(&value) }); err != nil {
				return stepArguments{}, err
			}
			return stepArguments{Method: value.Method, Path: value.Path}, nil
		default:
			return stepArguments{}, fmt.Errorf("editor method %q is invalid", method)
		}
	case "executor":
		var value struct {
			Command    string    `json:"command"`
			Arguments  *[]string `json:"args"`
			WorkingDir string    `json:"cwd,omitempty"`
		}
		if err := decodeStrict(string(raw), func(decoder *json.Decoder) error { return decoder.Decode(&value) }); err != nil {
			return stepArguments{}, fmt.Errorf("parse executor arguments: %w", err)
		}
		if value.Arguments == nil {
			return stepArguments{}, fmt.Errorf("executor arguments are missing required string array args")
		}
		return stepArguments{Command: value.Command, Arguments: *value.Arguments, WorkingDir: value.WorkingDir}, nil
	case "tester":
		var value struct {
			URL            string `json:"url"`
			ScreenshotPath string `json:"screenshotPath"`
		}
		if err := decodeStrict(string(raw), func(decoder *json.Decoder) error { return decoder.Decode(&value) }); err != nil {
			return stepArguments{}, fmt.Errorf("parse tester arguments: %w", err)
		}
		return stepArguments{URL: value.URL, ScreenshotPath: value.ScreenshotPath}, nil
	case "search":
		var value struct {
			Query string `json:"query"`
		}
		if err := decodeStrict(string(raw), func(decoder *json.Decoder) error { return decoder.Decode(&value) }); err != nil {
			return stepArguments{}, fmt.Errorf("parse search arguments: %w", err)
		}
		return stepArguments{Query: value.Query}, nil
	case "github":
		method, err := parseMethod(raw, "GitHub")
		if err != nil {
			return stepArguments{}, err
		}
		switch method {
		case "clone":
			var value struct {
				Method        string `json:"method"`
				RepositoryURL string `json:"repoUrl"`
				Destination   string `json:"destination"`
			}
			if err := decodeToolArguments(raw, "GitHub clone", func(decoder *json.Decoder) error { return decoder.Decode(&value) }); err != nil {
				return stepArguments{}, err
			}
			return stepArguments{Method: value.Method, RepositoryURL: value.RepositoryURL, Destination: value.Destination}, nil
		case "createBranch":
			var value struct {
				Method     string `json:"method"`
				Owner      string `json:"owner"`
				Repository string `json:"repo"`
				Branch     string `json:"branch"`
				Base       string `json:"base,omitempty"`
			}
			err := decodeToolArguments(raw, "GitHub createBranch", func(decoder *json.Decoder) error {
				return decoder.Decode(&value)
			})
			if err != nil {
				return stepArguments{}, err
			}
			return stepArguments{Method: value.Method, Owner: value.Owner, Repository: value.Repository, Branch: value.Branch, Base: value.Base}, nil
		case "commitAndPush":
			var value struct {
				Method  string `json:"method"`
				Path    string `json:"path"`
				Branch  string `json:"branch"`
				Message string `json:"message"`
			}
			err := decodeToolArguments(raw, "GitHub commitAndPush", func(decoder *json.Decoder) error {
				return decoder.Decode(&value)
			})
			if err != nil {
				return stepArguments{}, err
			}
			return stepArguments{Method: value.Method, Path: value.Path, Branch: value.Branch, Message: value.Message}, nil
		case "createPR":
			var value struct {
				Method     string `json:"method"`
				Owner      string `json:"owner"`
				Repository string `json:"repo"`
				Title      string `json:"title"`
				Body       string `json:"body"`
				Head       string `json:"head"`
				Base       string `json:"base,omitempty"`
			}
			if err := decodeToolArguments(raw, "GitHub createPR", func(decoder *json.Decoder) error { return decoder.Decode(&value) }); err != nil {
				return stepArguments{}, err
			}
			return stepArguments{
				Method: value.Method, Owner: value.Owner, Repository: value.Repository, Title: value.Title,
				Body: value.Body, Head: value.Head, Base: value.Base,
			}, nil
		default:
			return stepArguments{}, fmt.Errorf("GitHub method %q is invalid", method)
		}
	default:
		return stepArguments{}, fmt.Errorf("tool %q is invalid; expected editor, executor, tester, search, or github", tool)
	}
}

func parseMethod(raw json.RawMessage, label string) (string, error) {
	var selector struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(raw, &selector); err != nil {
		return "", fmt.Errorf("parse %s method: %w", label, err)
	}
	if selector.Method == "" {
		return "", fmt.Errorf("%s method is empty", label)
	}
	return selector.Method, nil
}

func decodeToolArguments(raw json.RawMessage, label string, decode func(*json.Decoder) error) error {
	if err := decodeStrict(string(raw), decode); err != nil {
		return fmt.Errorf("parse %s arguments: %w", label, err)
	}
	return nil
}

func decodeStrict(content string, decode func(*json.Decoder) error) error {
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	decoder.DisallowUnknownFields()
	if err := decode(decoder); err != nil {
		return fmt.Errorf("expected one strict JSON value: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("expected one strict JSON value, received trailing value %s", trailing)
		}
		return fmt.Errorf("check trailing JSON data: %w", err)
	}
	return nil
}

func validateInstruction(instruction stepInstruction) error {
	args := instruction.Args
	switch instruction.Tool {
	case "editor":
		if args.Path == "" {
			return fmt.Errorf("editor instruction path is empty")
		}
		switch args.Method {
		case "writeFile", "readFile":
			return nil
		case "applyPatch":
			if args.Search == "" {
				return fmt.Errorf("editor applyPatch search block is empty")
			}
			return nil
		default:
			return fmt.Errorf("editor method %q is invalid; expected writeFile, readFile, or applyPatch", args.Method)
		}
	case "executor":
		if args.Command == "" {
			return fmt.Errorf("executor command is empty")
		}
		return nil
	case "tester":
		if args.URL == "" || args.ScreenshotPath == "" {
			return fmt.Errorf("tester instruction requires non-empty url and screenshotPath")
		}
		return nil
	case "search":
		if strings.TrimSpace(args.Query) == "" {
			return fmt.Errorf("search query is empty")
		}
		return nil
	case "github":
		switch args.Method {
		case "clone":
			if args.RepositoryURL == "" || args.Destination == "" {
				return fmt.Errorf("github clone requires non-empty repoUrl and destination")
			}
		case "createBranch":
			if args.Owner == "" || args.Repository == "" || args.Branch == "" {
				return fmt.Errorf("github createBranch requires non-empty owner, repo, and branch")
			}
		case "commitAndPush":
			if args.Path == "" || args.Branch == "" || strings.TrimSpace(args.Message) == "" {
				return fmt.Errorf("github commitAndPush requires non-empty path, branch, and message")
			}
		case "createPR":
			if args.Owner == "" || args.Repository == "" || args.Title == "" || args.Head == "" {
				return fmt.Errorf("github createPR requires non-empty owner, repo, title, and head")
			}
		default:
			return fmt.Errorf("github method %q is invalid; expected clone, createBranch, commitAndPush, or createPR", args.Method)
		}
		return nil
	default:
		return fmt.Errorf("tool %q is invalid; expected editor, executor, tester, search, or github", instruction.Tool)
	}
}

func (o *Orchestrator) GeneratePlan(prompt string) ([]types.PlanStep, error) {
	const planningPrompt = "You are a senior technical architect. Create a detailed, multi-stage plan to solve the user's task. " +
		"Ensure each step is actionable and verifiable. Return only a valid JSON array of strings."
	o.history = []llm.Message{
		{Role: "system", Content: planningPrompt},
		{Role: "user", Content: prompt},
	}

	content, err := o.llm.Chat(o.history)
	if err != nil {
		return nil, fmt.Errorf("generate plan: %w", err)
	}
	o.history = append(o.history, llm.Message{Role: "assistant", Content: content})

	steps, err := parsePlan(content)
	if err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}

	o.plan = make([]types.PlanStep, len(steps))
	for i, s := range steps {
		o.plan[i] = types.PlanStep{ID: i + 1, Description: s, Status: "pending"}
	}
	return o.plan, nil
}

func (o *Orchestrator) ExecutePlan(onComplete func(types.PlanStep)) error {
	for i := range o.plan {
		step := &o.plan[i]
		var success bool
		for retries := 0; !success && retries < 3; retries++ {
			result, err := o.executeStep(step)
			if err != nil {
				fmt.Printf("Step %d error: %v\n", step.ID, err)
				continue
			}
			v, err := o.validator.Validate(step.Description, result)
			if err != nil || !v.Valid {
				feedback := v.Feedback
				if err != nil {
					feedback = err.Error()
				}
				fmt.Printf("Validation failed for step %d: %s\n", step.ID, feedback)
				o.history = append(o.history, llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("Step %q failed validation. Feedback: %s. Please try again.", step.Description, feedback),
				})
				continue
			}
			resultJSON, err := json.Marshal(result)
			if err != nil {
				return fmt.Errorf("encode result for completed step %d: %w", step.ID, err)
			}
			o.history = append(o.history, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Step %q result: %s", step.Description, resultJSON),
			})
			success = true
		}
		if !success {
			step.Status = "failed"
			return fmt.Errorf("step %d failed after 3 retries", step.ID)
		}
		step.Status = "completed"
		if onComplete != nil {
			onComplete(*step)
		}
	}
	return nil
}

type stepInstruction struct {
	Thought string        `json:"thought"`
	Tool    string        `json:"tool"`
	Args    stepArguments `json:"args"`
}

type stepArguments struct {
	Method         string   `json:"method,omitempty"`
	Path           string   `json:"path,omitempty"`
	Content        string   `json:"content,omitempty"`
	Search         string   `json:"search,omitempty"`
	Replace        string   `json:"replace,omitempty"`
	Command        string   `json:"command,omitempty"`
	Arguments      []string `json:"args,omitempty"`
	WorkingDir     string   `json:"cwd,omitempty"`
	URL            string   `json:"url,omitempty"`
	ScreenshotPath string   `json:"screenshotPath,omitempty"`
	Query          string   `json:"query,omitempty"`
	RepositoryURL  string   `json:"repoUrl,omitempty"`
	Destination    string   `json:"destination,omitempty"`
	Owner          string   `json:"owner,omitempty"`
	Repository     string   `json:"repo,omitempty"`
	Branch         string   `json:"branch,omitempty"`
	Base           string   `json:"base,omitempty"`
	Message        string   `json:"message,omitempty"`
	Title          string   `json:"title,omitempty"`
	Body           string   `json:"body,omitempty"`
	Head           string   `json:"head,omitempty"`
}

func (o *Orchestrator) executeStep(step *types.PlanStep) (types.StepResult, error) {
	const sysPrompt = "You are a junior engineer executing a task. Think step-by-step. " +
		"Decide which tool to use. Available tools: editor, executor, tester, search, github. " +
		`Return JSON { "thought": "your reasoning", "tool": "name", "args": { ... } }`

	msgs := append(append([]llm.Message(nil), o.history...),
		llm.Message{Role: "system", Content: sysPrompt},
		llm.Message{Role: "user", Content: "Next step: " + step.Description},
	)

	content, err := o.llm.Chat(msgs)
	if err != nil {
		return types.StepResult{}, err
	}

	instr, err := parseStepInstruction(content)
	if err != nil {
		return types.StepResult{}, fmt.Errorf("parse step instruction: %w", err)
	}
	fmt.Printf("Agent Thought: %s\n", instr.Thought)

	switch instr.Tool {
	case "editor":
		switch instr.Args.Method {
		case "writeFile":
			return types.StepResult{Summary: "File written successfully"}, o.editor.WriteFile(instr.Args.Path, instr.Args.Content)
		case "applyPatch":
			err := o.editor.ApplyPatch(instr.Args.Path, instr.Args.Search, instr.Args.Replace)
			return types.StepResult{Summary: "Patch applied successfully"}, err
		case "readFile":
			output, err := o.editor.ReadFile(instr.Args.Path)
			return types.StepResult{Summary: "File read successfully", Output: output}, err
		}
	case "executor":
		execution, err := o.executor.Execute(instr.Args.Command, instr.Args.Arguments, instr.Args.WorkingDir)
		return types.StepResult{Summary: "Command completed", Execution: &execution}, err
	case "tester":
		output, err := o.tester.RunTest(instr.Args.URL, instr.Args.ScreenshotPath)
		return types.StepResult{Summary: "Browser check completed", Output: output}, err
	case "search":
		output, err := o.search.Search(instr.Args.Query)
		return types.StepResult{Summary: "Search completed", Output: output}, err
	case "github":
		switch instr.Args.Method {
		case "clone":
			err := o.github.CloneRepository(instr.Args.RepositoryURL, instr.Args.Destination)
			return types.StepResult{Summary: "Repository cloned"}, err
		case "createBranch":
			err := o.github.CreateBranch(instr.Args.Owner, instr.Args.Repository, instr.Args.Branch, instr.Args.Base)
			return types.StepResult{Summary: "Branch created"}, err
		case "commitAndPush":
			err := o.github.CommitAndPush(instr.Args.Path, instr.Args.Branch, instr.Args.Message)
			return types.StepResult{Summary: "Repository changes committed and pushed"}, err
		case "createPR":
			pullRequest, err := o.github.CreatePullRequest(
				instr.Args.Owner,
				instr.Args.Repository,
				instr.Args.Title,
				instr.Args.Body,
				instr.Args.Head,
				instr.Args.Base,
			)
			return types.StepResult{Summary: "Pull request created", PullRequest: &pullRequest}, err
		}
	}
	return types.StepResult{}, fmt.Errorf("unknown tool/method: %s/%s", instr.Tool, instr.Args.Method)
}

func (o *Orchestrator) GetPlan() []types.PlanStep {
	return o.plan
}
