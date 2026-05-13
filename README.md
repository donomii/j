# Junior Engineer AI Agent

An autonomous, asynchronous AI coding agent designed to act as a "junior engineer" teammate.

## Architecture

- **Task Manager**: Handles task initiation via GitHub issues or webhooks.
- **Orchestrator**: Manages the agentic flow: Planning -> Approval -> Execution -> Review.
- **Sandbox Environment**: Isolated environment for running code and tests.
- **GitHub Client**: Tightly integrates with GitHub for branch management and PRs.
- **Toolbox**:
  - Code Editor: Refactoring and editing multiple files.
  - Test Runner: Executing tests and capturing results.
  - Browser: Playwright for frontend testing.
  - Web Search: Proactive web surfing for documentation.

## Tech Stack

- **Language**: TypeScript/Node.js
- **LLM**: Integration with OpenAI/Anthropic (via LangChain or similar)
- **Sandbox**: Docker / VM
- **API**: GitHub API

## Getting Started

(Instructions to follow)
