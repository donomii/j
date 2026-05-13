import { Octokit } from "octokit";
import { spawnSync } from "child_process";
import * as fs from "fs";

export class GitHubService {
  private octokit: Octokit;

  constructor(token: string) {
    this.octokit = new Octokit({ auth: token });
  }

  private sanitize(str: string): string {
    // Basic sanitization to prevent command injection
    return str.replace(/[^a-zA-Z0-9.\-_:/]/g, "");
  }

  async cloneRepository(repoUrl: string, destination: string) {
    const safeUrl = repoUrl; // URL should be validated more thoroughly in prod
    const safeDest = this.sanitize(destination);
    if (fs.existsSync(safeDest)) {
      fs.rmSync(safeDest, { recursive: true, force: true });
    }
    const res = spawnSync("git", ["clone", safeUrl, safeDest]);
    if (res.status !== 0) {
      throw new Error(`Failed to clone repository: ${res.stderr.toString()}`);
    }
  }

  async createBranch(repoOwner: string, repoName: string, branchName: string, baseBranch: string = "main") {
    const owner = this.sanitize(repoOwner);
    const repo = this.sanitize(repoName);
    const branch = this.sanitize(branchName);

    const { data: baseRef } = await this.octokit.rest.git.getRef({
      owner,
      repo,
      ref: `heads/${baseBranch}`,
    });

    await this.octokit.rest.git.createRef({
      owner,
      repo,
      ref: `refs/heads/${branch}`,
      sha: baseRef.object.sha,
    });
  }

  async commitAndPush(repoPath: string, branchName: string, message: string) {
    const path = this.sanitize(repoPath);
    const branch = this.sanitize(branchName);

    spawnSync("git", ["-C", path, "add", "."]);
    spawnSync("git", ["-C", path, "commit", "-m", message]);
    const res = spawnSync("git", ["-C", path, "push", "origin", branch]);
    if (res.status !== 0) {
      throw new Error(`Failed to push: ${res.stderr.toString()}`);
    }
  }

  async createPullRequest(owner: string, repo: string, title: string, body: string, head: string, base: string = "main") {
    const { data: pr } = await this.octokit.rest.pulls.create({
      owner: this.sanitize(owner),
      repo: this.sanitize(repo),
      title,
      body,
      head: this.sanitize(head),
      base: this.sanitize(base),
    });
    return pr;
  }
}
