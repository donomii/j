# J

J is an interactive and issue-driven coding agent. It asks a language model for a strict plan, presents the plan for approval in interactive mode, and executes each approved step through defined tools.

## Setup

Requirements:

- Go 1.22 or newer for the primary application.
- Node.js 20 or newer for the TypeScript implementation and browser tooling.
- Docker when command execution is enabled. Commands run in a container with only the approved workspace mounted.

Install the TypeScript dependencies:

```fish
npm ci
```

Copy `.env.example` to `.env`, then edit the values in `.env`. The checked-in example contains every setting and its normal default.

## Run

Start the primary interactive application with no flags:

```fish
./launch.fish
```

The same application can be started with `./run.sh` or `go run .`.

Build both implementations with `./build.sh`. Run all deterministic checks with `./test.sh`. `./demo.sh` starts the interactive application. `./install.sh` installs dependencies and the Go executable.

The TypeScript implementation can also be run with:

```fish
npm run build
npm start
```

## Settings

- `J_WORKSPACE`: the only host directory the model-selected editor, repository, screenshot, and command tools may use. The default is the directory where J starts. Set it to the repository or workspace J should work on.
- `USE_DOCKER`: controls command execution. The default is `true`, which runs commands inside a container. When set to `false`, the executor is disabled; commands never fall back to unrestricted host execution.
- `J_EXECUTOR_IMAGE`: container image used for commands. The default is `node:20-slim`. Choose an image containing the build tools required by the approved workspace.
- `LLM_PROVIDER`: language-model provider. The Go application defaults to `local`; supported remote values are `openai` and `ollama`.
- `LLM_MODEL_NAME`: model name sent to the selected provider. Provider-specific defaults apply when it is empty.
- `OPENAI_API_KEY`: required when `LLM_PROVIDER=openai`.
- `OLLAMA_BASE_URL`: Ollama server URL. The default is `http://localhost:11434`.
- `MODEL_URL`: model archive used by the Go local provider. The application uses its built-in default when this is empty.
- `GITHUB_TOKEN`: required for GitHub issue, branch, push, and pull-request operations.
- `REPO_OWNER` and `REPO_NAME`: repository polled by the background application.

## Tool boundaries

Editor paths may be relative to `J_WORKSPACE` or absolute paths inside it. Parent traversal and symlink escapes are rejected. `writeFile` creates a new file and refuses to replace an existing file; `applyPatch` requires exactly one matching block.

Executor instructions contain an executable and an argument array. J passes them separately to the configured container and disables container networking. It does not reconstruct a command line.

Repository clones accept canonical HTTPS GitHub URLs and refuse existing destinations. Repository staging, commit, and push failures are returned at the operation that failed.

Plans, validation results, and tool instructions must be one strict JSON value matching the documented schema. Fenced, prefixed, suffixed, incomplete, or undeclared fields are rejected.
