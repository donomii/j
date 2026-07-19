import { spawn } from "child_process";
import * as path from "path";
import { ExecutionResult } from "../types.js";
import { Workspace } from "../workspace.js";

export class SandboxExecutor {
  private workspace: Workspace;
  private useDocker: boolean;
  private image: string;

  constructor(workspace: Workspace, useDocker: boolean, image: string) {
    this.workspace = workspace;
    this.useDocker = useDocker;
    this.image = image;
  }

  async execute(command: string, args: string[] = [], cwd?: string): Promise<ExecutionResult> {
    if (command === "") throw new Error("Executor command is empty.");
    if (!this.useDocker) {
      throw new Error(
        `Execute ${JSON.stringify(command)} failed: container execution is disabled; ` +
        "set USE_DOCKER=true to confine commands to the approved workspace."
      );
    }
    return this.executeInDocker(command, args, cwd);
  }

  private async executeInDocker(command: string, args: string[] = [], cwd?: string): Promise<ExecutionResult> {
    const dockerArgs = buildDockerArguments(this.workspace, this.image, command, args, cwd);
    return this.runSpawn("docker", dockerArgs);
  }

  private runSpawn(command: string, args: string[] = [], cwd?: string): Promise<ExecutionResult> {
    return new Promise((resolve, reject) => {
      const child = spawn(command, args, { cwd });
      let stdout = "";
      let stderr = "";

      child.stdout.on("data", (data) => {
        stdout += data.toString();
      });

      child.stderr.on("data", (data) => {
        stderr += data.toString();
      });

      child.on("close", (code) => {
        resolve({
          stdout,
          stderr,
          exitCode: code ?? 1,
        });
      });

      child.on("error", (err) => {
        reject(new Error(`Start executable ${JSON.stringify(command)} with ${args.length} arguments: ${err.message}`));
      });
    });
  }
}

export function buildDockerArguments(
  workspace: Workspace,
  image: string,
  command: string,
  args: string[],
  cwd?: string
): string[] {
  const resolvedDirectory = workspace.resolve(cwd || workspace.root, true);
  const relativeDirectory = path.relative(workspace.root, resolvedDirectory);
  const containerDirectory = path.posix.join("/workspace", relativeDirectory.split(path.sep).join(path.posix.sep));
  return [
    "run", "--rm", "--network=none",
    "-v", `${workspace.root}:/workspace`,
    "-w", containerDirectory,
    image,
    command,
    ...args
  ];
}
