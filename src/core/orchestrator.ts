import { ChatOpenAI } from "@langchain/openai";
import { ChatOllama } from "@langchain/ollama";
import { BaseChatModel } from "@langchain/core/language_models/chat_models";
import { HumanMessage, SystemMessage, BaseMessage, AIMessage } from "@langchain/core/messages";
import { GitHubService } from "../github/service.js";
import { SandboxExecutor } from "../sandbox/executor.js";
import { CodeEditor } from "../tools/editor.js";
import { PlaywrightTester } from "../tools/tester.js";
import { WebSearchTool } from "../tools/search.js";
import { Validator } from "./validator.js";
import { parsePlanResponse, parseStepInstruction } from "./contracts.js";
import { OrchestratorConfig, PlanStep, StepResult } from "../types.js";
import { Workspace } from "../workspace.js";

export class Orchestrator {
  private llm: BaseChatModel;
  private github: GitHubService;
  private executor: SandboxExecutor;
  private editor: CodeEditor;
  private tester: PlaywrightTester;
  private searchTool: WebSearchTool;
  private validator: Validator;
  private plan: PlanStep[] = [];
  private history: BaseMessage[] = [];

  constructor(config: OrchestratorConfig) {
    if (config.provider === "openai") {
      this.llm = new ChatOpenAI({ openAIApiKey: config.apiKey, modelName: config.modelName || "gpt-4", temperature: 0 });
    } else {
      this.llm = new ChatOllama({
        baseUrl: config.baseUrl || "http://localhost:11434",
        model: config.modelName || "llama3",
        temperature: 0
      });
    }
    const workspace = new Workspace(config.workspaceRoot);
    this.github = new GitHubService(config.githubToken, workspace);
    this.executor = new SandboxExecutor(workspace, config.useDocker, config.executorImage);
    this.editor = new CodeEditor(workspace);
    this.tester = new PlaywrightTester(workspace);
    this.searchTool = new WebSearchTool();
    this.validator = new Validator(this.llm);
  }

  async generatePlan(prompt: string): Promise<PlanStep[]> {
    this.history = [
      new SystemMessage(
        "You are a senior technical architect. Create a detailed, multi-stage plan to solve the user's task. " +
        "Ensure each step is actionable and verifiable. Return only a valid JSON array of strings."
      ),
      new HumanMessage(prompt)
    ];

    const response = await this.llm.invoke(this.history);
    this.history.push(new AIMessage(response.content as string));

    const content = typeof response.content === 'string' ? response.content : JSON.stringify(response.content);
    const steps = parsePlanResponse(content);

    this.plan = steps.map((step: string, index: number) => ({
      id: index + 1,
      description: step,
      status: "pending",
    }));

    return this.plan;
  }

  async executePlan(onStepComplete?: (step: PlanStep) => void) {
    for (const step of this.plan) {
      let retryCount = 0;
      let stepSuccess = false;

      while (!stepSuccess && retryCount < 3) {
        try {
          const result = await this.executeStep(step);
          const validation = await this.validator.validateStepOutput(step.description, result);

          if (validation.valid) {
            this.history.push(new HumanMessage(`Step "${step.description}" result: ${JSON.stringify(result)}`));
            stepSuccess = true;
          } else {
            console.log(`Validation failed for step ${step.id}: ${validation.feedback}`);
            this.history.push(
              new HumanMessage(
                `Step "${step.description}" failed validation. ` +
                `Feedback: ${validation.feedback}. Please try again.`
              )
            );
            retryCount++;
          }
        } catch (error) {
          console.error(`Step ${step.id} execution error:`, error);
          retryCount++;
        }
      }

      if (stepSuccess) {
        step.status = "completed";
      } else {
        step.status = "failed";
        throw new Error(`Step ${step.id} failed after ${retryCount} retries.`);
      }

      if (onStepComplete) {
        onStepComplete(step);
      }
    }
  }

  private async executeStep(step: PlanStep): Promise<StepResult> {
    const systemPrompt = "You are a junior engineer executing a task. Think step-by-step. " +
        "Decide which tool to use. Available tools: editor, executor, tester, search, github. " +
        "Return JSON { \"thought\": \"your reasoning\", \"tool\": \"name\", \"args\": { ... } }";

    const messages = [
        ...this.history,
        new SystemMessage(systemPrompt),
        new HumanMessage(`Next step: ${step.description}`)
    ];

    const response = await this.llm.invoke(messages);
    const content = typeof response.content === 'string' ? response.content : JSON.stringify(response.content);
    const instruction = parseStepInstruction(content);

    console.log(`Agent Thought: ${instruction.thought}`);

    switch (instruction.tool) {
      case "editor":
        const editorArgs = instruction.args;
        if (editorArgs.method === "writeFile") {
            this.editor.writeFile(editorArgs.path, editorArgs.content);
            return { summary: "File written successfully" };
        } else if (editorArgs.method === "applyPatch") {
            this.editor.applyPatch(editorArgs.path, editorArgs.search, editorArgs.replace);
            return { summary: "Patch applied successfully" };
        }
        return { summary: "File read successfully", output: this.editor.readFile(editorArgs.path) };
      case "executor":
        return {
          summary: "Command completed",
          execution: await this.executor.execute(
            instruction.args.command,
            instruction.args.args,
            instruction.args.cwd
          )
        };
      case "tester":
        return {
          summary: "Browser check completed",
          output: await this.tester.runTest(instruction.args.url, instruction.args.screenshotPath)
        };
      case "search":
        return { summary: "Search completed", output: await this.searchTool.search(instruction.args.query) };
      case "github":
        const githubArgs = instruction.args;
        if (githubArgs.method === "clone") {
            await this.github.cloneRepository(githubArgs.repoUrl, githubArgs.destination);
            return { summary: "Repository cloned" };
        } else if (githubArgs.method === "createBranch") {
            await this.github.createBranch(githubArgs.owner, githubArgs.repo, githubArgs.branch, githubArgs.base);
            return { summary: "Branch created" };
        } else if (githubArgs.method === "commitAndPush") {
            await this.github.commitAndPush(githubArgs.path, githubArgs.branch, githubArgs.message);
            return { summary: "Repository changes committed and pushed" };
        }
        return {
          summary: "Pull request created",
          pullRequest: await this.github.createPullRequest(
            githubArgs.owner,
            githubArgs.repo,
            githubArgs.title,
            githubArgs.body,
            githubArgs.head,
            githubArgs.base
          )
        };
    }
  }

  getPlan() {
    return this.plan;
  }
}
