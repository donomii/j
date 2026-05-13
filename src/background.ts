import { Octokit } from "octokit";
import { Orchestrator } from "./core/orchestrator.js";
import * as dotenv from "dotenv";

dotenv.config();

async function pollIssues() {
  const githubToken = process.env.GITHUB_TOKEN;
  const owner = process.env.REPO_OWNER;
  const repo = process.env.REPO_NAME;
  const apiKey = process.env.OPENAI_API_KEY || "";

  if (!githubToken || !owner || !repo) {
    console.error("Missing configuration for polling issues.");
    return;
  }

  const octokit = new Octokit({ auth: githubToken });
  const orchestrator = new Orchestrator({
    provider: (process.env.LLM_PROVIDER as "openai" | "ollama") || "openai",
    apiKey,
    githubToken
  });

  console.log(`Polling issues for ${owner}/${repo}...`);

  while (true) {
    try {
      const { data: issues } = await octokit.rest.issues.listForRepo({
        owner,
        repo,
        state: "open",
        labels: "agent-task",
      });

      for (const issue of issues) {
        console.log(`Starting task from issue #${issue.number}: ${issue.title}`);

        // Remove the label so we don't process it again
        await octokit.rest.issues.removeLabel({
          owner,
          repo,
          issue_number: issue.number,
          name: "agent-task",
        });

        await octokit.rest.issues.createComment({
          owner,
          repo,
          issue_number: issue.number,
          body: "I've picked up this task! I'm starting the planning phase now.",
        });

        try {
          const plan = await orchestrator.generatePlan(issue.body || issue.title);
          const planText = plan.map(s => `${s.id}. ${s.description}`).join("\n");

          await octokit.rest.issues.createComment({
            owner,
            repo,
            issue_number: issue.number,
            body: `Here is my proposed plan:\n\n${planText}\n\nI will now begin execution.`,
          });

          await orchestrator.executePlan();

          await octokit.rest.issues.createComment({
            owner,
            repo,
            issue_number: issue.number,
            body: "I've completed the task! Please review the changes in the repository.",
          });

          await octokit.rest.issues.update({
            owner,
            repo,
            issue_number: issue.number,
            state: "closed",
          });
        } catch (error: any) {
          await octokit.rest.issues.createComment({
            owner,
            repo,
            issue_number: issue.number,
            body: `I encountered an error while performing the task: ${error.message}`,
          });
        }
      }
    } catch (error: any) {
      console.error("Error polling issues:", error.message);
    }

    await new Promise(resolve => setTimeout(resolve, 60000)); // Poll every minute
  }
}

pollIssues().catch(console.error);
