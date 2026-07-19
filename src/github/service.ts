import { Octokit } from "octokit";
import { spawnSync } from "child_process";
import * as fs from "fs";
import { PullRequestSummary } from "../types.js";
import { Workspace } from "../workspace.js";

const namePattern = /^[a-zA-Z0-9][a-zA-Z0-9._-]*$/;
const branchPattern = /^[a-zA-Z0-9](?:[a-zA-Z0-9._/-]*[a-zA-Z0-9])?$/;

export class GitHubService {
  private octokit: Octokit;

  constructor(token: string, private workspace: Workspace) {
    this.octokit = new Octokit({ auth: token });
  }

  private validateName(label: string, value: string): string {
    if (!namePattern.test(value)) {
      throw new Error(
        `${label} ${JSON.stringify(value)} is invalid; expected letters, numbers, dot, underscore, or hyphen.`
      );
    }
    return value;
  }

  private validateBranch(label: string, value: string): string {
    const invalid = !branchPattern.test(value) || value.includes("..") || value.includes("//") ||
      value.includes("@{") || value.endsWith(".lock");
    if (invalid) {
      throw new Error(`${label} ${JSON.stringify(value)} is invalid; expected a safe Git branch name.`);
    }
    return value;
  }

  async cloneRepository(repoUrl: string, destination: string) {
    const safeUrl = validateRepositoryUrl(repoUrl);
    const safeDest = this.workspace.resolve(destination, false);
    if (fs.existsSync(safeDest)) {
      throw new Error(`Clone destination ${JSON.stringify(safeDest)} already exists and will not be replaced.`);
    }
    this.runGit("clone repository", ["clone", safeUrl, safeDest]);
  }

  async createBranch(repoOwner: string, repoName: string, branchName: string, baseBranch: string = "main") {
    const owner = this.validateName("Repository owner", repoOwner);
    const repo = this.validateName("Repository name", repoName);
    const branch = this.validateBranch("Branch name", branchName);
    const base = this.validateBranch("Base branch", baseBranch);

    const { data: baseRef } = await this.octokit.rest.git.getRef({
      owner,
      repo,
      ref: `heads/${base}`,
    });

    await this.octokit.rest.git.createRef({
      owner,
      repo,
      ref: `refs/heads/${branch}`,
      sha: baseRef.object.sha,
    });
  }

  async commitAndPush(repoPath: string, branchName: string, message: string) {
    const repo = this.workspace.resolve(repoPath, true);
    const branch = this.validateBranch("Branch name", branchName);
    if (message.trim() === "") throw new Error(`Commit repository ${JSON.stringify(repo)} failed: commit message is empty.`);
    this.runGit("stage repository changes", ["-C", repo, "add", "."]);
    this.runGit("commit repository changes", ["-C", repo, "commit", "-m", message]);
    this.runGit("push repository branch", ["-C", repo, "push", "origin", branch]);
  }

  async createPullRequest(
    owner: string,
    repo: string,
    title: string,
    body: string,
    head: string,
    base: string = "main"
  ): Promise<PullRequestSummary> {
    const { data: pr } = await this.octokit.rest.pulls.create({
      owner: this.validateName("Repository owner", owner),
      repo: this.validateName("Repository name", repo),
      title,
      body,
      head: this.validateBranch("Head branch", head),
      base: this.validateBranch("Base branch", base),
    });
    if (pr.number <= 0 || !pr.html_url) {
      throw new Error(
        `Created pull request returned invalid number=${pr.number} html_url=${JSON.stringify(pr.html_url)}.`
      );
    }
    return { number: pr.number, htmlUrl: pr.html_url };
  }

  private runGit(task: string, args: string[]): void {
    const result = spawnSync("git", args, { encoding: "utf-8" });
    if (result.error) throw new Error(`${task} could not start: ${result.error.message}`);
    if (result.status !== 0) {
      throw new Error(
        `${task} failed with exit code ${result.status}; stderr=${JSON.stringify(result.stderr.trim())}; ` +
        `stdout=${JSON.stringify(result.stdout.trim())}`
      );
    }
  }
}

export function validateRepositoryUrl(rawUrl: string): string {
  const parsed = new URL(rawUrl);
  const parts = parsed.pathname.match(/^\/([^/]+)\/([^/]+?)(?:\.git)?$/);
  const invalid = parsed.protocol !== "https:" || parsed.host !== "github.com" || parsed.username ||
    parsed.password || parsed.search || parsed.hash || !parts;
  if (invalid || !parts) {
    throw new Error(`Repository URL ${JSON.stringify(rawUrl)} is invalid; expected https://github.com/OWNER/REPOSITORY.git.`);
  }
  if (!namePattern.test(parts[1]) || !namePattern.test(parts[2])) {
    throw new Error(`Repository URL ${JSON.stringify(rawUrl)} contains an invalid owner or repository name.`);
  }
  return `https://github.com/${parts[1]}/${parts[2]}.git`;
}
