# 当前进展与 Roadmap

> 最近更新：2026-06-11
> 模块层定义见 [module-map.md](./module-map.md)。✅ = 已落地，🟡 = 简版/部分，❌ = 未开始。
> **本工程自身的目标架构（事件流为中心的 D 层设计）见
> [docs/architecture/agent-core.md](../architecture/agent-core.md)**，Roadmap 与其实施路线对齐。

## 已完成对照表

| 层 | 状态 | 工程位置 | 说明 |
|---|---|---|---|
| A 配置 | ✅ | `infra/config/` | yaml + settings 加载 |
| A 账号/凭据 | ✅ | `domain/account/` | file + db 两种 loader，provider 抽象，凭据运行期查询 |
| A 可观测性 | ❌ | — | tracing / metrics 未做 |
| B 模型层 | ✅ | `domain/llm/` + `internal/llm/langchaingo/` | vendored langchaingo：openai / anthropic / gemini，含流式 |
| C 工具 registry | ✅ | `internal/tools/registry.go` | 线程安全注册表 + 工具定义导出 |
| C 内置工具 | 🟡 | `internal/tools/builtin/` | calculator / search / file / terminal / browser（search 偏 stub） |
| C 沙箱 | 🟡 | `internal/tools/sandbox/` | 本地沙箱已做，远程沙箱未做 |
| C 工具流式 | 🟡 | `internal/tools/streaming/` | router + json parser + 几个 stream handler |
| D 主循环 | 🟡 | `cmd/agent/agent_runtime.go` | **已跑通** function-calling 循环；将按目标架构重写为事件驱动（壳在 `internal/agent/`） |
| D 事件流 | 🟡 | `internal/event/` | 事件类型 + Sink 接口 + console 壳（思想来自 agent-server："一切皆事件"） |
| D 记忆/上下文 | 🟡 | `internal/task/` + `internal/agent/assembler.go` | 重构为 task_id→事件历史 + 上下文组装；Store 接口与内存壳、组装器壳已建，滑动窗口未做 |
| D 提示词 | 🟡 | `internal/prompt/templates/` | 分段系统提示词模板已提炼（system/loop/planner/todo/error）；渲染器未做；`templates.go` 旧模板待废弃 |
| D 计划层 | 🟡 | `internal/task/plan.go` | Plan/Phase 数据结构已建；plan_update/plan_advance 元工具未做 |
| D 鲁棒性 | ❌ | — | 修复链（重复检测、非法工具、坏参数）未做，方案见架构文档 §4.4 |
| 输入边界 | 🟡 | `internal/input/` | Source 接口 + stdin 壳（E/F 的替身，交互形态未定） |
| ~~E 会话与调度~~ | — | — | 不做，收缩为 `input.Source` 接口 |
| ~~F 服务端~~ | — | — | 不做，见 principles.md |

## 一句话现状

目标架构已定稿（事件流为中心，见 architecture/agent-core.md），骨架包的接口壳全部就位；
**最大的缺口是主循环还没改造成事件驱动**，记忆/计划/修复链都等这一步打底。

## Roadmap（与 architecture/agent-core.md §8 实施路线一致）

> 方向以 [principles.md](./principles.md) 为准：只做 agent 本质（B/C/D），
> **E 会话调度 / F 服务端已明确砍掉**，收缩为 input.Source / event.Sink / task.Store 三接口。

1. **M1 事件流打底** — event/task 包从壳变实，主循环重写为事件驱动
   （迁移现有 function-calling 逻辑），ConsoleSink 直出过程。 ← *推荐先做*
2. **M2 上下文组装** — Assembler 实现事件→消息完整映射，同 task_id 续聊跑通多轮对话。
3. **M3 REPL + 流式** — input.Source 升级为交互循环，接通 B 层已有流式能力。
4. **M4 提示词装配** — PromptRenderer 按段渲染 `internal/prompt/templates/`。
5. **M5 计划层** — PlanManager + plan_update/plan_advance/task_done 元工具。
6. **M6 修复链** — 按架构文档 §4.4 逐个实现修复器（重复检测、非法工具、坏参数…）。
7. **M7 滑动窗口** — Assembler 补上下文压缩。
8. **M8 持久化** — task.Store 换文件/SQLite 实现，获得退出后 resume。

> 另有独立项：工具增强 + 高风险命令权限确认（C 层，可与 M 系列并行）。
> 下一步具体做哪个，由我（用户）后续再定。

## 维护约定

- 每完成一个模块，更新上面的对照表（状态 + 位置 + 说明）和"一句话现状"。
- 顶层 `CLAUDE.md` 的 Architecture 应与此表保持一致，发现不符就同步。
