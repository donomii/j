import { BaseChatModel } from "@langchain/core/language_models/chat_models";
import { HumanMessage, SystemMessage } from "@langchain/core/messages";

export class Validator {
  constructor(private llm: BaseChatModel) {}

  async validateStepOutput(step: string, output: any): Promise<{ valid: boolean; feedback?: string }> {
    const systemPrompt = "You are a quality assurance engineer. Review the output of a task step. " +
        "Determine if the output is correct and moves towards the goal. " +
        "Return JSON { \"valid\": boolean, \"feedback\": \"string\" }";

    const userPrompt = `Step: ${step}\nOutput: ${JSON.stringify(output)}`;

    const response = await this.llm.invoke([
        new SystemMessage(systemPrompt),
        new HumanMessage(userPrompt)
    ]);

    const content = typeof response.content === 'string' ? response.content : JSON.stringify(response.content);
    return this.parseJsonResponse(content);
  }

  private parseJsonResponse(content: string): any {
    const jsonMatch = content.match(/```json\n([\s\S]*?)\n```/) || content.match(/```([\s\S]*?)```/);
    const rawJson = jsonMatch ? jsonMatch[1] : content;
    try {
        return JSON.parse(rawJson!.trim());
    } catch (e) {
        const cleaned = rawJson!.trim().replace(/^[^[{]*/, "").replace(/[^\]}]*$/, "");
        return JSON.parse(cleaned);
    }
  }
}
