import * as fs from "fs";
import * as path from "path";

export class CodeEditor {
  readFile(filepath: string): string {
    return fs.readFileSync(filepath, "utf-8");
  }

  writeFile(filepath: string, content: string): void {
    const dir = path.dirname(filepath);
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
    fs.writeFileSync(filepath, content, "utf-8");
  }

  /**
   * Applies a patch using exact block replacement.
   * This is safer than a global string replacement.
   */
  applyPatch(filepath: string, searchBlock: string, replaceBlock: string): void {
    const content = this.readFile(filepath);
    if (!content.includes(searchBlock)) {
      throw new Error(`Search block not found in ${filepath}`);
    }

    // Ensure the block is unique or handle multiple occurrences carefully.
    const occurrences = content.split(searchBlock).length - 1;
    if (occurrences > 1) {
       console.warn(`Warning: Multiple occurrences of search block in ${filepath}. Replacing all.`);
    }

    const newContent = content.split(searchBlock).join(replaceBlock);
    this.writeFile(filepath, newContent);
  }

  listFiles(dir: string): string[] {
    let results: string[] = [];
    const list = fs.readdirSync(dir);
    for (const file of list) {
      if (file === 'node_modules' || file === '.git' || file === 'dist') continue;
      const fullPath = path.join(dir, file);
      const stat = fs.statSync(fullPath);
      if (stat && stat.isDirectory()) {
        results = results.concat(this.listFiles(fullPath));
      } else {
        results.push(fullPath);
      }
    }
    return results;
  }
}
