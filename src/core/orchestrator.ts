import { ChatOpenAI } from "@langchain/openai";
import { HumanMessage, SystemMessage, BaseMessage, AIMessage } from "@langchain/core/messages";
import { GitHubService } from "../github/service.js";
import { SandboxExecutor } from "../sandbox/executor.js";
import { CodeEditor } from "../tools/editor.js";
import { PlaywrightTester } from "../tools/tester.js";
import { WebSearchTool } from "../tools/search.js";

export interface PlanStep {
  id: number;
  description: string;
  status: "pending" | "completed" | "failed";
}

export class Orchestrator {
  private llm: ChatOpenAI;
  private github: GitHubService;
  private executor: SandboxExecutor;
  private editor: CodeEditor;
  private tester: PlaywrightTester;
  private searchTool: WebSearchTool;
  private plan: PlanStep[] = [];
  private history: BaseMessage[] = [];

  constructor(apiKey: string, githubToken: string) {
    this.llm = new ChatOpenAI({ openAIApiKey: apiKey, modelName: "gpt-4", temperature: 0 });
    this.github = new GitHubService(githubToken);
    this.executor = new SandboxExecutor();
    this.editor = new CodeEditor();
    this.tester = new PlaywrightTester();
    this.searchTool = new WebSearchTool();
  }

  private parseJsonResponse(content: string): any {
    const jsonMatch = content.match(/```json\n([\s\S]*?)\n```/) || content.match(/```([\s\S]*?)```/);
    const rawJson = jsonMatch ? jsonMatch[1] : content;
    return JSON.parse(rawJson!.trim());
  }

  async generatePlan(prompt: string): Promise<PlanStep[]> {
    this.history = [
      new SystemMessage(
        "You are a junior engineer AI. Given a task, break it down into a list of actionable steps. " +
        "Return the plan as a JSON array of strings."
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
      try {
        const result = await this.executeStep(step);
        this.history.push(new HumanMessage(`Step "${step.description}" result: ${JSON.stringify(result)}`));
        step.status = "completed";
      } catch (error) {
        step.status = "failed";
        throw error;
      }
      if (onStepComplete) {
        onStepComplete(step);
      }
    }
  }

  private async executeStep(step: PlanStep) {
    const systemPrompt = "Decide which tool to use. Return JSON { \"tool\": \"...\", \"args\": { ... } }";
    const messages = [
        ...this.history,
        new SystemMessage(systemPrompt),
        new HumanMessage(`Next step to execute: ${step.description}`)
    ];

    const response = await this.llm.invoke(messages);
    const content = typeof response.content === 'string' ? response.content : JSON.stringify(response.content);
    const { tool, args } = this.parseJsonResponse(content);

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
}
