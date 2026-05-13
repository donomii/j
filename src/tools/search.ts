import axios from "axios";

export class WebSearchTool {
  /**
   * Performs a web search using the DuckDuckGo HTML interface.
   * This is a simple, no-API-key-required implementation for a junior agent.
   */
  async search(query: string): Promise<string> {
    try {
      const response = await axios.get(`https://html.duckduckgo.com/html/?q=${encodeURIComponent(query)}`, {
        headers: {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
        }
      });

      // Simple extraction of snippet-like text (very rough for this prototype)
      const text = response.data as string;
      const results = text.match(/<a class="result__snippet"[\s\S]*?>([\s\S]*?)<\/a>/g);

      if (!results) return "No results found.";

      return results.slice(0, 3).map(r => r.replace(/<[^>]*>?/gm, '').trim()).join("\n---\n");
    } catch (error: any) {
      return `Search failed: ${error.message}`;
    }
  }
}
