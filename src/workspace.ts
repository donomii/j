import * as fs from "fs";
import * as path from "path";

export class Workspace {
  readonly root: string;

  constructor(selectedRoot: string) {
    const resolved = fs.realpathSync(path.resolve(selectedRoot));
    if (!fs.statSync(resolved).isDirectory()) {
      throw new Error(`Workspace ${JSON.stringify(resolved)} is not a directory.`);
    }
    this.root = resolved;
  }

  resolve(selectedPath: string, mustExist: boolean): string {
    if (selectedPath === "") throw new Error("Workspace path is empty.");
    if (hasParentTraversal(selectedPath)) {
      throw new Error(`Workspace path ${JSON.stringify(selectedPath)} contains parent traversal.`);
    }
    const candidate = path.resolve(this.root, selectedPath);
    const resolved = resolveExistingPrefix(candidate);
    if (!contains(this.root, resolved)) {
      throw new Error(
        `Path ${JSON.stringify(selectedPath)} resolves outside approved workspace ${JSON.stringify(this.root)}.`
      );
    }
    if (mustExist && !fs.existsSync(resolved)) {
      throw new Error(`Required workspace path ${JSON.stringify(selectedPath)} does not exist.`);
    }
    return resolved;
  }
}

function hasParentTraversal(selectedPath: string): boolean {
  return selectedPath.split(/[\\/]/).includes("..");
}

function contains(root: string, candidate: string): boolean {
  const relative = path.relative(root, candidate);
  return relative !== ".." && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
}

function resolveExistingPrefix(candidate: string): string {
  let existing = candidate;
  while (true) {
    try {
      fs.lstatSync(existing);
      break;
    } catch (error) {
      if (!(error instanceof Error) || !("code" in error) || error.code !== "ENOENT") throw error;
      const parent = path.dirname(existing);
      if (parent === existing) throw new Error(`No existing ancestor for workspace path ${JSON.stringify(candidate)}.`);
      existing = parent;
    }
  }
  const resolvedPrefix = fs.realpathSync(existing);
  return path.resolve(resolvedPrefix, path.relative(existing, candidate));
}
