package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/donomii/j/types"
	"github.com/donomii/j/workspace"
)

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
var branchRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._/-]*[a-zA-Z0-9])?$`)
var repositoryPathRe = regexp.MustCompile(`^/([^/]+)/([^/]+?)(?:\.git)?$`)

type Service struct {
	token         string
	client        *http.Client
	workspaceRoot types.WorkspaceRoot
}

func New(token string, workspaceRoot types.WorkspaceRoot) *Service {
	return &Service{token: token, client: &http.Client{}, workspaceRoot: workspaceRoot}
}

func validateName(label, value string) (string, error) {
	if !nameRe.MatchString(value) {
		return "", fmt.Errorf("%s %q is invalid; expected letters, numbers, dot, underscore, or hyphen", label, value)
	}
	return value, nil
}

func validateBranch(label, value string) (string, error) {
	invalid := !branchRe.MatchString(value) || strings.Contains(value, "..") || strings.Contains(value, "//") ||
		strings.Contains(value, "@{") || strings.HasSuffix(value, ".lock")
	if invalid {
		return "", fmt.Errorf("%s %q is invalid; expected a safe Git branch name", label, value)
	}
	return value, nil
}

func validateRepository(owner, repo string) (string, string, error) {
	validatedOwner, err := validateName("repository owner", owner)
	if err != nil {
		return "", "", err
	}
	validatedRepo, err := validateName("repository name", repo)
	if err != nil {
		return "", "", err
	}
	return validatedOwner, validatedRepo, nil
}

func validateRepositoryURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("repository URL %q is invalid: %w", rawURL, err)
	}
	pathParts := repositoryPathRe.FindStringSubmatch(parsed.EscapedPath())
	invalid := parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || len(pathParts) != 3
	if invalid {
		return "", fmt.Errorf("repository URL %q is invalid; expected https://github.com/OWNER/REPOSITORY.git", rawURL)
	}
	if _, err := validateName("repository owner", pathParts[1]); err != nil {
		return "", err
	}
	if _, err := validateName("repository name", pathParts[2]); err != nil {
		return "", err
	}
	return "https://github.com/" + pathParts[1] + "/" + pathParts[2] + ".git", nil
}

func runGit(repoPath, task string, args ...string) error {
	commandArgs := append([]string{"-C", repoPath}, args...)
	out, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s in %q: %w: %s", task, repoPath, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *Service) apiDo(method, path string, body map[string]string) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode GitHub request body for %s %s: %w", method, path, err)
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, "https://api.github.com"+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github API %s %s: %d %s", method, path, resp.StatusCode, data)
	}
	return data, nil
}

func (s *Service) CloneRepository(repoURL, destination string) error {
	validatedURL, err := validateRepositoryURL(repoURL)
	if err != nil {
		return err
	}
	dest, err := workspace.Resolve(s.workspaceRoot, destination, false)
	if err != nil {
		return fmt.Errorf("clone repository into %q: %w", destination, err)
	}
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("clone repository into %q: destination already exists and will not be replaced", dest)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect clone destination %q: %w", dest, err)
	}
	out, err := exec.Command("git", "clone", validatedURL, dest).CombinedOutput()
	if err != nil {
		return fmt.Errorf("clone repository %q into %q: %w: %s", validatedURL, dest, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *Service) CreateBranch(owner, repo, branch, base string) error {
	if base == "" {
		base = "main"
	}
	var err error
	if owner, err = validateName("repository owner", owner); err != nil {
		return err
	}
	if repo, err = validateName("repository name", repo); err != nil {
		return err
	}
	if branch, err = validateBranch("branch name", branch); err != nil {
		return err
	}
	if base, err = validateBranch("base branch", base); err != nil {
		return err
	}

	refData, err := s.apiDo("GET", fmt.Sprintf("/repos/%s/%s/git/ref/heads/%s", owner, repo, base), nil)
	if err != nil {
		return err
	}
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.Unmarshal(refData, &ref); err != nil {
		return err
	}
	_, err = s.apiDo("POST", fmt.Sprintf("/repos/%s/%s/git/refs", owner, repo), map[string]string{
		"ref": "refs/heads/" + branch,
		"sha": ref.Object.SHA,
	})
	return err
}

func (s *Service) CommitAndPush(repoPath, branch, message string) error {
	resolvedPath, err := workspace.Resolve(s.workspaceRoot, repoPath, true)
	if err != nil {
		return fmt.Errorf("commit repository %q: %w", repoPath, err)
	}
	branch, err = validateBranch("branch name", branch)
	if err != nil {
		return err
	}
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("commit repository %q: commit message is empty", resolvedPath)
	}
	if err := runGit(resolvedPath, "stage repository changes", "add", "."); err != nil {
		return err
	}
	if err := runGit(resolvedPath, "commit repository changes", "commit", "-m", message); err != nil {
		return err
	}
	if err := runGit(resolvedPath, "push repository branch", "push", "origin", branch); err != nil {
		return err
	}
	return nil
}

func (s *Service) CreatePullRequest(owner, repo, title, body, head, base string) (types.PullRequestSummary, error) {
	if base == "" {
		base = "main"
	}
	var err error
	if owner, err = validateName("repository owner", owner); err != nil {
		return types.PullRequestSummary{}, err
	}
	if repo, err = validateName("repository name", repo); err != nil {
		return types.PullRequestSummary{}, err
	}
	if head, err = validateBranch("head branch", head); err != nil {
		return types.PullRequestSummary{}, err
	}
	if base, err = validateBranch("base branch", base); err != nil {
		return types.PullRequestSummary{}, err
	}
	data, err := s.apiDo("POST", fmt.Sprintf("/repos/%s/%s/pulls", owner, repo), map[string]string{
		"title": title,
		"body":  body,
		"head":  head,
		"base":  base,
	})
	if err != nil {
		return types.PullRequestSummary{}, err
	}
	var result struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return types.PullRequestSummary{}, fmt.Errorf("decode created pull request for %s/%s: %w", owner, repo, err)
	}
	if result.Number <= 0 || result.HTMLURL == "" {
		return types.PullRequestSummary{}, fmt.Errorf(
			"decode created pull request for %s/%s: expected positive number and html_url, received number=%d html_url=%q",
			owner, repo, result.Number, result.HTMLURL,
		)
	}
	return types.PullRequestSummary{Number: result.Number, HTMLURL: result.HTMLURL}, nil
}

func (s *Service) IssueComment(owner, repo string, number int, body string) error {
	owner, repo, err := validateRepository(owner, repo)
	if err != nil {
		return err
	}
	if number <= 0 {
		return fmt.Errorf("issue number %d is invalid; expected a positive integer", number)
	}
	_, err = s.apiDo("POST", fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number), map[string]string{"body": body})
	return err
}

func (s *Service) RemoveLabel(owner, repo string, number int, label string) error {
	owner, repo, err := validateRepository(owner, repo)
	if err != nil {
		return err
	}
	if number <= 0 {
		return fmt.Errorf("issue number %d is invalid; expected a positive integer", number)
	}
	label, err = validateName("issue label", label)
	if err != nil {
		return err
	}
	_, err = s.apiDo("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/labels/%s", owner, repo, number, label), nil)
	return err
}

func (s *Service) CloseIssue(owner, repo string, number int) error {
	owner, repo, err := validateRepository(owner, repo)
	if err != nil {
		return err
	}
	if number <= 0 {
		return fmt.Errorf("issue number %d is invalid; expected a positive integer", number)
	}
	_, err = s.apiDo("PATCH", fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, number), map[string]string{"state": "closed"})
	return err
}

type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

func (s *Service) ListIssues(owner, repo, label string) ([]Issue, error) {
	owner, repo, err := validateRepository(owner, repo)
	if err != nil {
		return nil, err
	}
	label, err = validateName("issue label", label)
	if err != nil {
		return nil, err
	}
	data, err := s.apiDo("GET", fmt.Sprintf("/repos/%s/%s/issues?state=open&labels=%s", owner, repo, label), nil)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	err = json.Unmarshal(data, &issues)
	return issues, err
}
