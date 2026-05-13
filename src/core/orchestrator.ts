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

export interface PlanStep {
  id: number;
  description: string;
  status: "pending" | "completed" | "failed";
}

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

  constructor(config: { provider: "openai" | "ollama", apiKey?: string, baseUrl?: string, modelName?: string, githubToken: string }) {
    if (config.provider === "openai") {
      this.llm = new ChatOpenAI({ openAIApiKey: config.apiKey, modelName: config.modelName || "gpt-4", temperature: 0 });
    } else {
      this.llm = new ChatOllama({ baseUrl: config.baseUrl || "http://localhost:11434", model: config.modelName || "llama3", temperature: 0 });
    }
    this.github = new GitHubService(config.githubToken);
    this.executor = new SandboxExecutor();
    this.editor = new CodeEditor();
    this.tester = new PlaywrightTester();
    this.searchTool = new WebSearchTool();
    this.validator = new Validator(this.llm);
  }

  private parseJsonResponse(content: string): any {
    const jsonMatch = content.match(/```json\n([\s\S]*?)\n```/) || content.match(/```([\s\S]*?)```/);
    const rawJson = jsonMatch ? jsonMatch[1] : content;
    try {
        return JSON.parse(rawJson!.trim());
    } catch (e) {
        const cleaned = rawJson!.trim().replace(/^[^[{]*/, "").replace(/[^\]}]*$/, "");
        return JSON.parse(cleaned);
    }
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
    const steps = this.parseJsonResponse(content);

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
            this.history.push(new HumanMessage(`Step "${step.description}" failed validation. Feedback: ${validation.feedback}. Please try again.`));
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

  private async executeStep(step: PlanStep) {
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
    const { thought, tool, args } = this.parseJsonResponse(content);

    console.log(`Agent Thought: ${thought}`);

    let result: any;
    switch (tool) {
      case "editor":
        if (args.method === "writeFile") {
            this.editor.writeFile(args.path, args.content);
            result = "File written successfully";
        } else if (args.method === "applyPatch") {
            this.editor.applyPatch(args.path, args.search, args.replace);
            result = "Patch applied successfully";
        } else if (args.method === "readFile") {
            result = this.editor.readFile(args.path);
        }
        break;
      case "executor":
        result = await this.executor.execute(args.command, args.args || [], args.cwd);
        break;
      case "tester":
        result = await this.tester.runTest(args.url, args.screenshotPath);
        break;
      case "search":
        result = await this.searchTool.search(args.query);
        break;
      case "github":
        if (args.method === "clone") {
            await this.github.cloneRepository(args.repoUrl, args.destination);
            result = "Repository cloned";
        } else if (args.method === "createBranch") {
            await this.github.createBranch(args.owner, args.repo, args.branch);
            result = "Branch created";
        } else if (args.method === "commitAndPush") {
            await this.github.commitAndPush(args.path, args.branch, args.message);
            result = "Code pushed";
        } else if (args.method === "createPR") {
            result = await this.github.createPullRequest(args.owner, args.repo, args.title, args.body, args.head, args.base);
        }
        break;
      default:
        throw new Error(`Unknown tool: ${tool}`);
    }
    return result;
  }

  getPlan() {
    return this.plan;
  }
}
