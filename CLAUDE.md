# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go-based AI agent framework inspired by LangChain and ReAct. Early-stage project — many components (LLM calls, ReAct executor, search tool, summary/vector memory) are stubbed with TODOs. Code comments and documentation are in Chinese.

## Build & Run Commands

```bash
make build          # Compile to bin/agent
make run            # Run main agent (cmd/agent/main.go)
make test           # Run all tests: go test -v ./...
make lint           # Run golangci-lint
make example        # Run examples/basic/main.go
make install        # go mod download && go mod tidy
make test-coverage  # Generate coverage.out and coverage.html
```

Run a single test: `go test -v -run TestName ./internal/tools/...`

## Architecture

The agent follows a layered design with three core interfaces defined in `pkg/types/types.go`:

- **`LLMClient`** — `Generate(ctx, []Message)` and `GenerateWithOptions(ctx, []Message, GenerateOptions)`. Implemented by `internal/llm.Client`, which dispatches by provider (openai/anthropic/azure).
- **`Memory`** — `Add`, `GetHistory`, `Clear`. Only `BufferMemory` (sliding window of N messages) is implemented in `internal/memory/buffer.go`.
- **`Tool`** — `Name`, `Description`, `Execute(ctx, input string)`. Tools register into `internal/tools.Registry` (thread-safe map). Currently: `Calculator` (basic two-operand arithmetic), `Search` (stub).

**Agent flow:** `agent.New()` creates an `Agent` holding LLM + Memory + Tools + Config, and wires up an `Executor`. `Agent.Run(ctx, input)` saves input to memory, delegates to `Executor.Execute()`, saves response. The executor is currently a stub.

**ReAct mode:** `internal/agent/react.go` defines `ReActAgent` which embeds `Agent` and implements the Thought→Action→Observation loop. `parseReActOutput()` extracts structured fields via regex. The loop itself is stubbed.

**Prompt templates:** `internal/prompt/templates.go` contains Go template strings for ReAct, ZeroShot, and ChainOfThought patterns. Templates use `{{.Tools}}`, `{{.ToolNames}}`, `{{.Question}}` placeholders.

**Configuration:** `config/config.yaml` for LLM/agent/memory/tools settings. API keys come from environment variables (see `.env.example`).

## Adding a New Tool

1. Create a file in `internal/tools/` implementing the `types.Tool` interface (Name, Description, Execute).
2. Register it via `toolRegistry.Register(myTool)` in the main entrypoint.

## Separate Example Module

`examples/gemini-stream/` has its own `go.mod` — it's a standalone program demonstrating Gemini function calling with streaming, not part of the main module.
