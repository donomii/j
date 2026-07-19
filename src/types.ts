export interface PlanStep {
  id: number;
  description: string;
  status: "pending" | "completed" | "failed";
}

export interface ExecutionResult {
  stdout: string;
  stderr: string;
  exitCode: number;
}

export interface PullRequestSummary {
  number: number;
  htmlUrl: string;
}

export interface StepResult {
  summary: string;
  output?: string;
  execution?: ExecutionResult;
  pullRequest?: PullRequestSummary;
}

export interface ValidationResult {
  valid: boolean;
  feedback: string;
}

export interface OrchestratorConfig {
  provider: "openai" | "ollama";
  apiKey?: string;
  baseUrl?: string;
  modelName?: string;
  githubToken: string;
  workspaceRoot: string;
  useDocker: boolean;
  executorImage: string;
}

export type EditorArguments =
  | { method: "writeFile"; path: string; content: string }
  | { method: "applyPatch"; path: string; search: string; replace: string }
  | { method: "readFile"; path: string };

export interface ExecutorArguments {
  command: string;
  args: string[];
  cwd?: string;
}

export interface TesterArguments {
  url: string;
  screenshotPath: string;
}

export interface SearchArguments {
  query: string;
}

export type GitHubArguments =
  | { method: "clone"; repoUrl: string; destination: string }
  | { method: "createBranch"; owner: string; repo: string; branch: string; base?: string }
  | { method: "commitAndPush"; path: string; branch: string; message: string }
  | {
      method: "createPR";
      owner: string;
      repo: string;
      title: string;
      body: string;
      head: string;
      base?: string;
    };

export type StepInstruction =
  | { thought: string; tool: "editor"; args: EditorArguments }
  | { thought: string; tool: "executor"; args: ExecutorArguments }
  | { thought: string; tool: "tester"; args: TesterArguments }
  | { thought: string; tool: "search"; args: SearchArguments }
  | { thought: string; tool: "github"; args: GitHubArguments };
