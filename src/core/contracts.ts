import { EditorArguments, GitHubArguments, StepInstruction, ValidationResult } from "../types.js";

type JsonObject = { [key: string]: JsonValue };
type JsonValue = string | number | boolean | null | JsonObject | JsonValue[];

export function parsePlanResponse(content: string): string[] {
  const value = parseJson(content, "plan");
  if (!Array.isArray(value) || value.length === 0) {
    throw new Error("Plan response must be a non-empty JSON array of non-empty strings.");
  }
  const steps: string[] = [];
  for (const step of value) {
    if (typeof step !== "string" || step.trim() === "") {
      throw new Error("Plan response must be a non-empty JSON array of non-empty strings.");
    }
    steps.push(step);
  }
  return steps;
}

export function parseStepInstruction(content: string): StepInstruction {
  const instruction = requireObject(parseJson(content, "step instruction"), "step instruction");
  requireExactKeys(instruction, ["thought", "tool", "args"], "step instruction");
  const thought = requireString(instruction, "thought", "step instruction");
  const tool = requireString(instruction, "tool", "step instruction");
  const args = requireObject(instruction.args, `${tool} arguments`);

  switch (tool) {
    case "editor":
      return { thought, tool, args: parseEditorArguments(args) };
    case "executor":
      requireExactKeys(args, ["command", "args", "cwd"], "executor arguments");
      return {
        thought,
        tool,
        args: {
          command: requireString(args, "command", "executor arguments"),
          args: requireStringArray(args, "args", "executor arguments"),
          cwd: optionalString(args, "cwd", "executor arguments")
        }
      };
    case "tester":
      requireExactKeys(args, ["url", "screenshotPath"], "tester arguments");
      return {
        thought,
        tool,
        args: {
          url: requireString(args, "url", "tester arguments"),
          screenshotPath: requireString(args, "screenshotPath", "tester arguments")
        }
      };
    case "search":
      requireExactKeys(args, ["query"], "search arguments");
      return { thought, tool, args: { query: requireString(args, "query", "search arguments") } };
    case "github":
      return { thought, tool, args: parseGitHubArguments(args) };
    default:
      throw new Error(`Step instruction tool ${JSON.stringify(tool)} is invalid.`);
  }
}

export function parseValidationResponse(content: string): ValidationResult {
  const value = requireObject(parseJson(content, "validation response"), "validation response");
  requireExactKeys(value, ["valid", "feedback"], "validation response");
  if (typeof value.valid !== "boolean") throw new Error("Validation response field valid must be boolean.");
  const feedback = requireString(value, "feedback", "validation response", value.valid);
  return { valid: value.valid, feedback };
}

function parseEditorArguments(args: JsonObject): EditorArguments {
  const method = requireString(args, "method", "editor arguments");
  if (method === "writeFile") {
    requireExactKeys(args, ["method", "path", "content"], "editor writeFile arguments");
    return {
      method,
      path: requireString(args, "path", "editor writeFile arguments"),
      content: requireString(args, "content", "editor writeFile arguments", true)
    };
  } else if (method === "applyPatch") {
    requireExactKeys(args, ["method", "path", "search", "replace"], "editor applyPatch arguments");
    return {
      method,
      path: requireString(args, "path", "editor applyPatch arguments"),
      search: requireString(args, "search", "editor applyPatch arguments"),
      replace: requireString(args, "replace", "editor applyPatch arguments", true)
    };
  } else if (method === "readFile") {
    requireExactKeys(args, ["method", "path"], "editor readFile arguments");
    return { method, path: requireString(args, "path", "editor readFile arguments") };
  }
  throw new Error(`Editor method ${JSON.stringify(method)} is invalid.`);
}

function parseGitHubArguments(args: JsonObject): GitHubArguments {
  const method = requireString(args, "method", "GitHub arguments");
  if (method === "clone") {
    requireExactKeys(args, ["method", "repoUrl", "destination"], "GitHub clone arguments");
    return {
      method,
      repoUrl: requireString(args, "repoUrl", "GitHub clone arguments"),
      destination: requireString(args, "destination", "GitHub clone arguments")
    };
  } else if (method === "createBranch") {
    requireExactKeys(args, ["method", "owner", "repo", "branch", "base"], "GitHub createBranch arguments");
    return {
      method,
      owner: requireString(args, "owner", "GitHub createBranch arguments"),
      repo: requireString(args, "repo", "GitHub createBranch arguments"),
      branch: requireString(args, "branch", "GitHub createBranch arguments"),
      base: optionalString(args, "base", "GitHub createBranch arguments")
    };
  } else if (method === "commitAndPush") {
    requireExactKeys(args, ["method", "path", "branch", "message"], "GitHub commitAndPush arguments");
    return {
      method,
      path: requireString(args, "path", "GitHub commitAndPush arguments"),
      branch: requireString(args, "branch", "GitHub commitAndPush arguments"),
      message: requireString(args, "message", "GitHub commitAndPush arguments")
    };
  } else if (method === "createPR") {
    requireExactKeys(args, ["method", "owner", "repo", "title", "body", "head", "base"], "GitHub createPR arguments");
    return {
      method,
      owner: requireString(args, "owner", "GitHub createPR arguments"),
      repo: requireString(args, "repo", "GitHub createPR arguments"),
      title: requireString(args, "title", "GitHub createPR arguments"),
      body: requireString(args, "body", "GitHub createPR arguments", true),
      head: requireString(args, "head", "GitHub createPR arguments"),
      base: optionalString(args, "base", "GitHub createPR arguments")
    };
  }
  throw new Error(`GitHub method ${JSON.stringify(method)} is invalid.`);
}

function parseJson(content: string, label: string): JsonValue {
  try {
    return JSON.parse(content) as JsonValue;
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error);
    throw new Error(`Expected one strict JSON value for ${label}: ${detail}`);
  }
}

function requireObject(value: JsonValue | undefined, label: string): JsonObject {
  if (value === null || Array.isArray(value) || typeof value !== "object") throw new Error(`${label} must be a JSON object.`);
  return value;
}

function requireExactKeys(value: JsonObject, allowed: string[], label: string): void {
  const unexpected = Object.keys(value).filter((key) => !allowed.includes(key));
  if (unexpected.length > 0) throw new Error(`${label} contains unexpected fields: ${unexpected.join(", ")}.`);
}

function requireString(value: JsonObject, key: string, label: string, allowEmpty = false): string {
  const field = value[key];
  if (typeof field !== "string" || (!allowEmpty && field.trim() === "")) {
    throw new Error(`${label} field ${key} must be ${allowEmpty ? "a string" : "a non-empty string"}.`);
  }
  return field;
}

function optionalString(value: JsonObject, key: string, label: string): string | undefined {
  const field = value[key];
  if (field === undefined) return undefined;
  if (typeof field !== "string" || field.trim() === "") {
    throw new Error(`${label} field ${key} must be a non-empty string when provided.`);
  }
  return field;
}

function requireStringArray(value: JsonObject, key: string, label: string): string[] {
  const field = value[key];
  if (!Array.isArray(field) || !field.every((entry) => typeof entry === "string")) {
    throw new Error(`${label} field ${key} must be an array of strings.`);
  }
  return field;
}
