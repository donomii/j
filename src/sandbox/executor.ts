import { spawn } from "child_process";

export interface ExecutionResult {
  stdout: string;
  stderr: string;
  exitCode: number | null;
}

/**
 * SandboxExecutor handles command execution.
 * In a production environment, this would interface with a Docker container or a dedicated VM.
 */
export class SandboxExecutor {
  private isSandboxed: boolean = false;

  constructor() {
    // Check if we are running in a known sandbox environment (e.g., Docker)
    this.isSandboxed = process.env.RUNNING_IN_SANDBOX === 'true';
  }

  async execute(command: string, args: string[] = [], cwd?: string): Promise<ExecutionResult> {
    if (!this.isSandboxed && process.env.STRICT_SANDBOX === 'true') {
      throw new Error("Execution blocked: Not in a secure sandbox environment.");
    }

    // For this prototype, we use spawn which is safer than exec,
    // but full isolation should be handled by the environment (e.g. Docker).
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
