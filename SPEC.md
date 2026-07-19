# J Behavior Specification

## Purpose

J turns a user task or labelled GitHub issue into an approved sequence of coding operations. It supports a Go application and a TypeScript application with the same security boundaries and response contracts.

## Configuration

Configuration is read from `.env` and the environment. `J_WORKSPACE` defaults to the startup directory. `USE_DOCKER` defaults to true. `J_EXECUTOR_IMAGE` defaults to `node:20-slim`. Every model-selected filesystem destination must resolve inside `J_WORKSPACE` after symbolic links are resolved.

## Interactive workflow

1. Read a task from the user.
2. Request a plan from the configured language model.
3. Accept only a non-empty JSON array of non-empty strings.
4. Display the ordered plan.
5. Execute nothing unless the user approves the plan.
6. For each step, request one defined tool instruction, execute it, validate the result, and retry the step at most three times.
7. Stop with an accurate error when a step cannot be completed.

## Response contracts

A tool instruction is one JSON object containing `thought`, `tool`, and `args`. Unknown fields, missing required fields, invalid field types, surrounding prose, multiple JSON values, and code fences are rejected.

Supported tools and arguments:

- `editor`: `writeFile(path, content)`, `readFile(path)`, or `applyPatch(path, search, replace)`.
- `executor`: `command`, string-array `args`, and optional `cwd`.
- `tester`: `url` and `screenshotPath`.
- `search`: `query`.
- `github`: `clone(repoUrl, destination)`, `createBranch(owner, repo, branch, base)`, `commitAndPush(path, branch, message)`, or `createPR(owner, repo, title, body, head, base)`.

A validation response is one JSON object with required boolean `valid` and string `feedback`. A rejected result requires non-empty feedback.

## Workspace confinement

Paths may be relative to the approved workspace or absolute within it. Lexical parent traversal and symbolic-link traversal outside the workspace are rejected. Missing targets are allowed only for operations that create a new path.

`writeFile` creates one new file and refuses an existing destination. `applyPatch` edits an existing file only when the search block occurs exactly once. File listing does not traverse symbolic links.

Browser screenshots, repository destinations, repository working directories, and executor working directories use the same workspace resolver.

## Command execution

Model-selected commands run only when container execution is enabled. The approved workspace is the only host directory mounted, at `/workspace`. Networking is disabled. The executable and each argument remain separate values. Disabling container execution disables the executor rather than enabling host execution.

## Repository operations

Clone URLs have the exact form `https://github.com/OWNER/REPOSITORY.git`; the suffix may be omitted in input and is canonicalized. Clone destinations must be absent. Existing destinations are never replaced.

Staging, committing, and pushing are distinct checked operations. An error identifies the operation, repository path, command result, and captured output. GitHub API errors retain the method, path, status, and response body.

## Verification

The Go checks cover lexical traversal, symbolic-link escape rejection, create-only editing, exact patch matching, container argument separation, strict plan/instruction/validation parsing, URL validation, and clone-destination preservation. TypeScript checks cover the same response contracts, lexical traversal, container argument separation, and exact repository URLs. Tests use local files and pure contract logic and do not require network access.
