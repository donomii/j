import assert from "node:assert/strict";
import test from "node:test";

import { parsePlanResponse, parseStepInstruction, parseValidationResponse } from "../dist/core/contracts.js";
import { validateRepositoryUrl } from "../dist/github/service.js";
import { buildDockerArguments } from "../dist/sandbox/executor.js";
import { Workspace } from "../dist/workspace.js";

test("strict response contracts reject repaired or undeclared data", () => {
  assert.deepEqual(parsePlanResponse('["inspect","change","verify"]'), ["inspect", "change", "verify"]);
  assert.throws(() => parsePlanResponse('```json\n["inspect"]\n```'));
  assert.throws(() => parsePlanResponse('["inspect"] trailing'));
  assert.throws(() => parseStepInstruction('{"thought":"run","tool":"executor","args":{"command":"go","args":[],"extra":true}}'));
  assert.throws(() => parseValidationResponse('{"valid":false,"feedback":""}'));
});

test("container command keeps executable and arguments separate", () => {
  const workspace = new Workspace(process.cwd());
  assert.throws(() => workspace.resolve("src/../package.json", true));
  const args = buildDockerArguments(workspace, "node:20-slim", "node", ["script.js", "argument with spaces", "; literal"], process.cwd());
  assert.deepEqual(args.slice(-4), ["node", "script.js", "argument with spaces", "; literal"]);
  assert.equal(args.includes("--network=none"), true);
  assert.throws(() => buildDockerArguments(workspace, "node:20-slim", "node", [], "/"));
});

test("repository URLs match the exact GitHub owner and repository form", () => {
  assert.equal(validateRepositoryUrl("https://github.com/donomii/j"), "https://github.com/donomii/j.git");
  assert.throws(() => validateRepositoryUrl("https://github.com/donomii/j/"));
  assert.throws(() => validateRepositoryUrl("https://github.com/donomii//j"));
  assert.throws(() => validateRepositoryUrl("https://github.com:444/donomii/j"));
});
