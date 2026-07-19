import { BaseChatModel } from "@langchain/core/language_models/chat_models";
import { HumanMessage, SystemMessage } from "@langchain/core/messages";
import { parseValidationResponse } from "./contracts.js";
import { StepResult, ValidationResult } from "../types.js";

export class Validator {
  constructor(private llm: BaseChatModel) {}

  async validateStepOutput(step: string, output: StepResult): Promise<ValidationResult> {
    const systemPrompt = "You are a quality assurance engineer. Review the output of a task step. " +
        "Determine if the output is correct and moves towards the goal. " +
        "Return JSON { \"valid\": boolean, \"feedback\": \"string\" }";

    const userPrompt = `Step: ${step}\nOutput: ${JSON.stringify(output)}`;

    const response = await this.llm.invoke([
        new SystemMessage(systemPrompt),
        new HumanMessage(userPrompt)
    ]);

    const content = typeof response.content === 'string' ? response.content : JSON.stringify(response.content);
    return parseValidationResponse(content);
  }
}
