import { spawn } from "child_process";
import * as path from "path";

export interface ExecutionResult {
  stdout: string;
  stderr: string;
  exitCode: number | null;
}

export class SandboxExecutor {
  private useDocker: boolean;

  constructor() {
    this.useDocker = process.env.USE_DOCKER === 'true';
  }

  async execute(command: string, args: string[] = [], cwd?: string): Promise<ExecutionResult> {
    if (this.useDocker) {
      return this.executeInDocker(command, args, cwd);
    }
    return this.runSpawn(command, args, cwd);
  }

  private async executeInDocker(command: string, args: string[] = [], cwd?: string): Promise<ExecutionResult> {
    const workDir = cwd || process.cwd();
    const dockerArgs = [
      "run",
      "--rm",
      "-v", `${path.resolve(workDir)}:/workspace`,
      "-w", "/workspace",
      "node:20-slim",
      "sh", "-c", `${command} ${args.join(" ")}`
    ];

    return this.runSpawn("docker", dockerArgs);
  }

  private runSpawn(command: string, args: string[] = [], cwd?: string): Promise<ExecutionResult> {
    return new Promise((resolve) => {
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
          exitCode: code,
        });
      });

      child.on("error", (err) => {
        resolve({
          stdout,
          stderr: err.message,
          exitCode: 1,
        });
      });
    });
  }
}
