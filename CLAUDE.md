# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## ⚠️ 必读规则（每次会话）

1. **禁品牌字眼**：代码/注释/文档/标识符/提交信息中不得出现参考工程相关的品牌字眼
   （要上传 GitHub）。**具体禁用词清单只在 gitignore 掉的 `.claude/local-notes.md`**，
   本文件不写出来。参考工程一律称 `agent-server`，其本地路径同样只在 local-notes.md。
2. **禁硬编码密钥**：代码中不得出现 `token`/`api-key` 等敏感值，凭据只走环境变量
   （见 `.env.example`），运行期由 `domain/account` 加载。

## 这是什么项目 / 进展在哪

**Alembic**（炼金蒸馏器，slogan：*distill the essence of every agent*）：用 Go 萃取
各种参考工程（如 `agent-server`）的核心思路，做一个能"吸收、提炼别的 agent、取长补短"的
**本地、单用户、单可执行文件的终端 agent**（形态对标 Claude Code）。
**项目说明、模块地图、当前进展、roadmap 全在 [`docs/progress/`](docs/progress/README.md)**——
开新会话先读那里。完成模块后同步更新 `docs/progress/status.md` 和下面的 Architecture。

## 纲领（决定取舍的总开关）

详见 [`docs/progress/principles.md`](docs/progress/principles.md)。要点：

- **只做 agent 的本质复杂度**：B 模型层 / C 工具层 / D Agent 核心（思考、工具、记忆、
  鲁棒性、提示词）。
- **明确砍掉附带复杂度**：E 会话调度 / F 服务端（多租户、WebSocket、worker 分发、
  鉴权…）——对终端单文件目标无用，不做。
- **Claude 的义务**：当用户要做 E/F 类工作、或有更聚焦本质的更小做法时，**主动提醒**
  "这偏离了'只做 agent 本质'的纲领"。引入新参考项目时，先按"萃取四步"分层再动手。

## Project Overview

Go-based AI agent framework inspired by `agent-server`, LangChain and ReAct.
Code comments and documentation are in Chinese.

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

> 进展状态以 `docs/progress/status.md` 为准；下面是结构概览。

分层（对应 `docs/progress/module-map.md` 的 6 层）：

- **基础设施** — `infra/config/`（yaml + settings 加载）；`domain/account/`（凭据加载，
  file/db 两种 loader，provider 抽象，凭据运行期查询）。
- **模型层** — `domain/llm/`（按 provider 分发的 LLM 服务）+ vendored `internal/llm/langchaingo/`
  （openai / anthropic / gemini，含流式）。统一入口 `Service.GenerateContent`。
- **工具层** — `internal/tools/`：`registry.go` 线程安全注册表；`builtin/`（calculator /
  search / file / terminal / browser）；`sandbox/`（本地沙箱）；`executor/`；`streaming/`。
  共享接口 `Tool`（Name / Description / Execute）定义在 `pkg/types/types.go`。
- **Agent 核心** — `cmd/agent/agent_runtime.go` 的 `Agent.Run` 实现 function-calling 风格的
  工具调用循环（已跑通，将按目标架构重写为事件驱动）。
  **目标架构见 [`docs/architecture/agent-core.md`](docs/architecture/agent-core.md)**，
  骨架包（接口壳已建）：`internal/event/`（事件流 + Sink 外化）、`internal/task/`
  （task_id→事件历史 + 计划）、`internal/agent/`（上下文组装）、`internal/input/`
  （输入边界 Source）、`internal/prompt/templates/`（分段系统提示词模板）。

**入口流程：** `cmd/agent/main.go` 初始化 config → account → llm → 工具 registry（带本地沙箱）
→ 构造 `Agent` → `Agent.Run(ctx, prompt)` 跑工具调用循环直到拿到最终答复或达到最大迭代。

**待做：** 主循环事件驱动改造、上下文组装实现、REPL/流式、计划元工具、修复链、
滑动窗口、持久化（实施路线 M1–M8 见架构文档 §8）。E 会话调度 / F 服务端不做。
详见 `docs/progress/status.md`。

**Configuration:** `config/config.yaml` for LLM/agent/memory/tools settings. API keys come from environment variables (see `.env.example`).

## Adding a New Tool

1. Create a file in `internal/tools/builtin/` implementing the `types.Tool` interface (Name, Description, Execute).
2. Register it in `internal/tools/builtin/register.go` (`NewBuiltinRegistry`).

## Separate Example Module

`examples/gemini-stream/` has its own `go.mod` — it's a standalone program demonstrating Gemini function calling with streaming, not part of the main module.
