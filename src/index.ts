import { Orchestrator } from "./core/orchestrator.js";
import * as dotenv from "dotenv";
import * as readline from "readline";

dotenv.config();

const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
});

function askQuestion(query: string): Promise<string> {
  return new Promise((resolve) => rl.question(query, resolve));
}

async function main() {
  const provider = (process.env.LLM_PROVIDER as "openai" | "ollama") || "openai";
  const apiKey = process.env.OPENAI_API_KEY || "";
  const baseUrl = process.env.OLLAMA_BASE_URL || "http://localhost:11434";
  const modelName = process.env.LLM_MODEL_NAME;
  const githubToken = process.env.GITHUB_TOKEN || "";
  const workspaceRoot = process.env.J_WORKSPACE || process.cwd();
  const useDocker = process.env.USE_DOCKER?.toLowerCase() !== "false";
  const executorImage = process.env.J_EXECUTOR_IMAGE || "node:20-slim";

  if (provider === "openai" && !apiKey) {
    console.error("Please set OPENAI_API_KEY in .env");
    process.exit(1);
  }

  const orchestrator = new Orchestrator({
    provider,
    apiKey,
    baseUrl,
    modelName,
    githubToken,
    workspaceRoot,
    useDocker,
    executorImage
  });

  const prompt = await askQuestion("What task should I perform? ");

  console.log(`Generating plan using ${provider}...`);
  const plan = await orchestrator.generatePlan(prompt);

  console.log("Proposed Plan:");
  plan.forEach((step) => console.log(`${step.id}. ${step.description}`));

  const approval = await askQuestion("Do you approve this plan? (yes/no) ");

  if (approval.toLowerCase() === "yes") {
    console.log("Executing plan...");
    await orchestrator.executePlan((step) => {
      console.log(`Step ${step.id} completed.`);
    });
    console.log("Task completed successfully.");
  } else {
    console.log("Plan rejected. Task aborted.");
  }

  rl.close();
}

main().catch(console.error);
