# 参考工程 `agent-server` 模块地图

本工程用 Go 一点一点重写参考工程 `agent-server`（一个生产级 agent monorepo）的核心思路。
为了能"一个模块一个模块地啃"，把参考工程压成 **6 个层**。需要某层细节时，再去
`.claude/local-notes.md` 里的速查表找参考工程对应源码位置。

> 命名规则：参考工程统一称 `agent-server`，不写品牌字眼（见 [rules.md](./rules.md)）。

## A 基础设施层（Infra）

- 配置加载、环境变量、可观测性（tracing / profiler / metrics）。
- 职责：把"运行这个系统需要的外部输入"统一管起来。

## B 模型层（LLM）

- 多 provider 分发（anthropic / openai / azure / bedrock / vertex）。
- 请求体转换、流式 chunk 解析、统一的 GenerateContent 接口。
- 职责：屏蔽各家 API 差异，对上层提供一个干净的"调模型"能力。

## C 工具层（Tools / Handlers + Sandbox）

- 各类工具：terminal、textEditor、file、search、browser，以及业务工具
  （media、deploy、webdev、slide、connector 等）。
- 沙箱：本地沙箱 + 远程沙箱（e2b），给工具一个隔离的执行环境。
- 职责：让 agent 能"动手做事"，并保证执行安全。

## D Agent 核心（大脑）

这是最关键、也最大的一层，内部可再拆：

- **主循环**：Thought → Action → Observation，function-calling 风格。
- **计划（plan）**：把任务拆成步骤，按步执行。
- **记忆 / 上下文**：滑动窗口、上下文裁剪，支撑多轮对话。
- **知识（knowledge）**：检索/注入外部知识。
- **提示词（prompts）**：系统提示词、模板渲染与解析。
- **事件总线（eventBus）**：把思考/工具进度/流式输出事件分发给上层。
- **鲁棒性**：循环检测、重复调用检测、非法工具调用修复、重试策略。

## E 会话与调度（Session & Dispatch）

- WebSocket 会话服务、会话数据、会话锁。
- 任务队列 + worker 绑定/心跳/重排队（把一个会话稳定地交给一个 worker 处理）。
- 职责：支撑多用户、多任务并发，且能容错恢复。

## F 服务端（Server）

- HTTP / RPC 接口、路由、鉴权、中间件。
- 职责：把整套能力暴露成对外服务。

---

## 啃的顺序建议

LLM（B）→ 工具（C）→ Agent 核心（D）已经是当前重点。
D 层内部从"记忆 → 提示词 → 事件/流式"逐步推进；E/F 不做（收缩为接口，见纲领）。
当前实际进展见 [status.md](./status.md)；本工程自身如何组织这些层，
见 [../architecture/agent-core.md](../architecture/agent-core.md)。
