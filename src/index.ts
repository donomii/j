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
  const apiKey = process.env.OPENAI_API_KEY || "";
  const githubToken = process.env.GITHUB_TOKEN || "";

  if (!apiKey) {
    console.error("Please set OPENAI_API_KEY in .env");
    process.exit(1);
  }

  const orchestrator = new Orchestrator(apiKey, githubToken);

  const prompt = await askQuestion("What task should I perform? ");

  console.log("Generating plan...");
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
