import * as fs from "fs";
import * as path from "path";
import { Workspace } from "../workspace.js";

export class CodeEditor {
  constructor(private workspace: Workspace) {}

  readFile(filepath: string): string {
    return fs.readFileSync(this.workspace.resolve(filepath, true), "utf-8");
  }

  writeFile(filepath: string, content: string): void {
    const resolved = this.workspace.resolve(filepath, false);
    const dir = path.dirname(resolved);
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
    fs.writeFileSync(resolved, content, { encoding: "utf-8", flag: "wx" });
  }

  /**
   * Applies a patch using exact block replacement.
   * This is safer than a global string replacement.
   */
  applyPatch(filepath: string, searchBlock: string, replaceBlock: string): void {
    const resolved = this.workspace.resolve(filepath, true);
    const content = fs.readFileSync(resolved, "utf-8");
    const occurrences = content.split(searchBlock).length - 1;
    if (occurrences !== 1) {
      throw new Error(
        `Apply patch to ${JSON.stringify(resolved)} expected exactly one search block, found ${occurrences}.`
      );
    }
    const newContent = content.replace(searchBlock, replaceBlock);
    fs.writeFileSync(resolved, newContent, "utf-8");
  }

  listFiles(dir: string): string[] {
    const resolvedDirectory = this.workspace.resolve(dir, true);
    let results: string[] = [];
    const list = fs.readdirSync(resolvedDirectory);
    for (const file of list) {
      if (file === 'node_modules' || file === '.git' || file === 'dist') continue;
      const fullPath = path.join(resolvedDirectory, file);
      const stat = fs.lstatSync(fullPath);
      if (stat.isSymbolicLink()) continue;
      if (stat && stat.isDirectory()) {
        results = results.concat(this.listFiles(fullPath));
      } else {
        results.push(fullPath);
      }
    }
    return results;
  }
}
