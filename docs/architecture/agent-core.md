# Alembic Agent 核心架构（目标设计 v1）

> 本文是 Alembic 的**目标架构**：以事件流为中心的本地终端 agent。
> 思想萃取自参考工程 `agent-server` 的 D 层（萃取过程见 `docs/progress/principles.md`），
> 但按"本地、单用户、单二进制"的北极星目标做了大幅裁剪与重构。
> **允许推翻现有代码**：`cmd/agent/agent_runtime.go` 的"消息数组 + for 循环"是过渡形态，
> 将按本文逐步重写。
>
> 各模块当前落地状态见 [`docs/progress/status.md`](../progress/status.md)。

---

## 1. 设计目标与边界

**要做的（本质复杂度，B/C/D）：**

- 一个事件驱动的 agent 主循环：思考 → 动作 → 观察，每轮一个工具。
- 显式计划层：agent 自己维护任务计划并按计划推进。
- 鲁棒性：响应校验 + 修复链（非法工具、重复调用、格式损坏）。
- 上下文组装：从任务事件历史生成 LLM 消息，预留滑动窗口压缩位。
- 模块化提示词：系统提示词按职责分段，独立成文件。

**砍掉的（附带复杂度，E/F），各自收缩成一个接口：**

| 参考工程里的样子 | Alembic 里的样子 |
|---|---|
| WebSocket 会话服务、分布式队列、会话锁、worker 心跳 | `input.Source` 接口：能 `Next()` 出一条用户消息即可（先用 stdin） |
| 事件经 Redis/Socket 广播到前端渲染 | `event.Sink` 接口：能 `Handle()` 一个事件即可（展示形态未定：CLI 直出或前端，先用 console 壳） |
| 数据库持久化 + checkpoint 恢复 | `task.Store` 接口：task_id → 事件历史（先用内存实现，持久化后补） |

> 这三个接口就是系统与外界的全部边界。换掉任何一侧的实现（stdin → REPL → HTTP），
> 核心循环一行不改。

---

## 2. 总体结构

```
                ┌─────────────────────────────────────────────┐
                │                  Agent 核心 (D)               │
 用户输入        │                                              │
──────────►  input.Source                                      │
                │   │ userMessage 事件                          │
                │   ▼                                          │
                │  ┌──────────── 事件流 (task.Store) ─────────┐ │
                │  │  task_id → [e1, e2, e3, ...] 只追加      │ │
                │  └──────┬──────────────────────────▲───────┘ │
                │         │ 触发迭代                  │ 追加事件 │
                │         ▼                          │         │
                │  ┌─────────────── 主循环 Loop ──────┴──────┐  │
                │  │ ① ContextAssembler  事件历史→LLM消息    │  │
                │  │ ② PromptRenderer    分段系统提示词       │  │
                │  │ ③ LLM 调用 ─────────────────► B 模型层  │  │
                │  │ ④ Validator+修复链  校验/纠错/重试       │  │
                │  │ ⑤ PlanManager       计划元工具拦截       │  │
                │  │ ⑥ Dispatcher        工具执行 ──► C 工具层│  │
                │  │ ⑦ Terminator        终止判定            │  │
                │  └────────────────────────────────────────┘  │
                │         │ 所有事件同步外化                     │
                │         ▼                                    │
 终端/前端 ◄── event.Sink                                       │
                └─────────────────────────────────────────────┘
```

核心思想（萃取自 `agent-server` 最重要的一条）：**循环的状态不是内存里的消息数组，
而是一条只追加的事件流**。记忆、上下文组装、外部展示、未来的持久化与恢复，
全部从同一条流派生：

- 上下文 = 事件流的序列化（+ 压缩）；
- 展示 = 事件流的实时投影；
- 恢复 = 按 task_id 重放事件流（暂不做，但结构上天然支持）。

---

## 3. 核心概念与数据结构

### 3.1 Event（事件）—— 一切行为皆事件

```go
type Event struct {
    ID      string    // 事件唯一 id
    TaskID  string    // 所属任务
    Type    Type      // 见下表
    Payload any       // 类型化负载，按 Type 区分
    Time    time.Time
}
```

| 事件类型 | 负载 | 谁产生 | 含义 |
|---|---|---|---|
| `user_message` | 文本 | input.Source | 用户输入，**触发迭代** |
| `action_start` | 思考文本 + 工具名 + 参数 | 主循环 | 一次工具调用决定（Thought+Action 合一） |
| `observation` | 工具输出 / 错误 | Dispatcher | 工具执行结果，**触发下一轮迭代** |
| `plan_update` | 完整计划 + 当前阶段 | PlanManager | 计划创建/修订/推进 |
| `assistant_message` | 文本 | 主循环 | 给用户的正式答复/交付 |
| `status` | 一句话 | 各模块 | 过程提示（"正在执行 shell"），仅外化不进上下文 |
| `task_done` | 结束原因 | Terminator | 任务结束（完成/上限/打断/出错） |

两条流转规则：

1. **userMessage 和 observation 是仅有的两种"迭代触发器"**——收到其一就跑一轮循环。
   这取代了 `for` 循环：循环的"下一圈"由事件驱动，而非控制流驱动。
2. **status 事件只进 Sink 不进上下文**——展示用的噪音不污染 LLM 的输入。

### 3.2 Task（任务）与 task_id

```go
type Task struct {
    ID     string        // task_id：上下文存取的唯一键
    Events []event.Event // 只追加的事件历史
    Plan   *Plan         // 当前计划（可为 nil）
}

type Store interface {
    Create(ctx context.Context) (*Task, error)
    Get(ctx context.Context, taskID string) (*Task, error)
    Append(ctx context.Context, taskID string, events ...event.Event) error
}
```

- 一次任务 = 一个 task_id = 一条事件流。多轮对话天然支持：下一条 userMessage
  追加到同一 task_id 即可。
- `Store` 当前只有内存实现（壳）；将来换文件/SQLite 实现即获得"退出后 resume"。

### 3.3 Plan（计划）

```go
type Plan struct {
    Phases  []Phase // 有序阶段
    Current int     // 当前阶段下标
}

type Phase struct {
    ID     int
    Title  string
    Status PhaseStatus // todo / doing / done
}
```

计划不靠独立的 planner agent，而是给主 agent 两个**元工具**（见 §4.5）。

---

## 4. 主循环：一次迭代的完整拆解

这是本架构最重要的部分。一轮迭代 = 一个触发事件进来，到产生下一批事件出去。

```
触发事件(userMessage | observation)
  → ⓪ 前置检查 → ① 上下文组装 → ② 提示词渲染 → ③ LLM 调用(内嵌④修复链)
  → ⑤ 计划元工具拦截 → ⑥ 工具分发执行 → ⑦ 终止判定
  → 产出事件(action_start / observation / plan_update / assistant_message / task_done)
```

### ⓪ 前置检查（Guard）

| 职责 | 检查迭代是否还该继续 |
|---|---|
| 输入 | 触发事件 + Task 状态 |
| 输出 | 继续 / 直接终止 |
| 检查项 | 用户是否已打断；迭代次数是否超上限（防失控的最后保险）；ctx 是否已取消 |

### ① 上下文组装（ContextAssembler）★ 本轮预留空壳

| 职责 | 把 task_id 的事件历史变成可直接喂给 LLM 的 `[]llms.MessageContent` |
|---|---|
| 输入 | task_id |
| 输出 | 消息列表 |

```go
type ContextAssembler interface {
    Assemble(ctx context.Context, taskID string) ([]llms.MessageContent, error)
}
```

映射规则（目标态）：

- `user_message` → user 消息
- `action_start` → assistant 消息（思考文本 + tool_call）
- `observation` → tool 消息（结果绑定 tool_call_id）
- `assistant_message` → assistant 消息
- `plan_update` → 以紧凑文本注入（让模型始终看到最新计划）
- `status` → **跳过**

预留的压缩位（暂不实现，接口已留）：当序列化总 token 超过窗口上限时，
把旧事件裁剪成 `--snip--` 占位、丢弃旧图片，但**保留所有 user_message 和最新
plan_update**。参考工程用"锚点 + 多维预算（总 token / snip token / 图片数）"实现，
将来照此补。

### ② 提示词渲染（PromptRenderer）

| 职责 | 用分段模板拼出系统提示词 |
|---|---|
| 输入 | 模板段 + 变量（语言、工具说明等） |
| 输出 | system 消息 |

系统提示词不是一坨大字符串，而是按职责分段、按需拼装（段文件在
`internal/prompt/templates/`，每段职责见该目录 README）：

```
system.md → agent_loop.md → planner.md → todo.md → error_handling.md
```

这样做的收益：改某一段不影响其他段；不同任务模式可以选择性装配段。

### ③ LLM 调用 + ④ 校验修复链（Validator & Repair Chain）

| 职责 | 调模型，并在**返回不可用时自动修复重试**，绝不把垃圾响应放进循环 |
|---|---|
| 输入 | 消息列表 + 工具定义 |
| 输出 | 一个合法的响应：纯文本（交付）或单个合法 tool_call |

修复链是包在 LLM 调用外面的中间件序列，每个修复器实现同一接口：

```go
// Verdict: pass（放行）/ retry（带修正后的请求重试）/ fail（彻底失败）
type Repair interface {
    Name() string
    Inspect(req *Request, resp *Response, history []event.Event) Verdict
}
```

按序计划实现的修复器（全部萃取自参考工程的 `fix*` 系列思想）：

| 修复器 | 触发条件 | 修复手段 |
|---|---|---|
| `singleToolCall` | 一次返回多个 tool_call | 只取第一个，其余丢弃（不算错误） |
| `unknownTool` | 调了不存在/被禁用的工具 | 注入"该工具不可用"提醒，重试 |
| `badArguments` | 参数 JSON 解析失败或缺必填 | 把解析错误注入为追加消息，重试 |
| `repetition` | 与最近 N 轮某次调用同名同参 | 注入"你在重复同一动作"系统提醒，重试；多次仍重复则终止并报告 |
| `emptyResponse` | 空响应 | 直接重试（带退避） |

设计决策：**修复优先于报错**。参考工程的经验是：模型的坏输出大多可以靠
"把错误描述喂回去"自愈，直接抛错中断任务是最后手段。

### ⑤ 计划元工具拦截（PlanManager）

| 职责 | 计划相关 tool_call 不进工具层，由核心自己消化 |
|---|---|
| 输入 | tool_call |
| 输出 | plan_update 事件 + 合成的 observation（告诉模型"计划已更新"） |

给模型注册三个**元工具**（不在 `tools.Registry` 里，由循环拦截）：

- `plan_update(phases, current_phase_id)` —— 创建或整体修订计划
- `plan_advance(phase_id)` —— 标记当前阶段完成，推进到下一阶段
- `task_done(message)` —— 显式宣布任务完成（见 ⑦）

提示词层（`planner.md` 段）要求：复杂任务必须先出计划；每完成一阶段调
`plan_advance`；目标变化时调 `plan_update` 修订。**计划的全局观（Plan-and-Solve）
就这样嫁接在逐轮反应（ReAct）的骨架上。**

### ⑥ 工具分发执行（Dispatcher）

| 职责 | 把合法 tool_call 交给工具层执行，结果变 observation 事件 |
|---|---|
| 输入 | tool_call |
| 输出 | observation 事件（成功或失败都**是事件不是异常**） |

流程：发 `action_start` 事件（思考文本 + 工具名 + 参数，Sink 立即可见）
→ `tools.Registry` 查找 → 沙箱内执行 → 结果（或错误文本）包成 `observation`
追加进事件流。**工具失败不中断循环**——错误作为 observation 喂回模型，
配合 `error_handling.md` 段提示词让模型自纠（先查参数 → 换方法 → 实在不行向用户求助）。

### ⑦ 终止判定（Terminator）

| 出口 | 条件 | 产出 |
|---|---|---|
| 正常交付 | 模型返回纯文本（无 tool_call），或调用 `task_done` | assistant_message + task_done(完成) |
| 迭代上限 | 本轮迭代数超 MaxIterations | task_done(上限)，附已完成进度 |
| 用户打断 | Source 侧收到中断信号 | task_done(打断) |
| 不可修复错误 | 修复链 fail / LLM 持续不可用 | task_done(出错)，错误入事件流 |

任务结束后 Task 留在 Store 里；同 task_id 再来 userMessage 即继续对话。

---

## 5. 模块拆分与包布局

| 包 | 职责 | 对应分层 | 状态 |
|---|---|---|---|
| `internal/event/` | 事件定义、Sink 接口、console 壳 | D 事件流 | 🟡 壳已建 |
| `internal/task/` | Task、task_id、Store 接口、内存实现 | D 记忆/上下文存储 | 🟡 壳已建 |
| `internal/agent/` | 主循环 Loop、ContextAssembler、修复链、PlanManager、Terminator | D 大脑 | 🟡 仅 Assembler 壳，循环待按本文重写 |
| `internal/input/` | Source 接口、stdin 实现 | 输入边界（E 的替身） | 🟡 壳已建 |
| `internal/prompt/` | 分段模板 + 渲染 | D 提示词 | 🟡 模板已提炼，渲染待做 |
| `internal/tools/` | 注册表、内置工具、沙箱、执行器 | C 工具层 | ✅ 沿用现有 |
| `domain/llm/` + vendored langchaingo | 多 provider 统一调用 | B 模型层 | ✅ 沿用现有 |
| `cmd/agent/` | 装配各模块 + 旧循环（待替换） | 入口 | ♻️ 待改造 |

依赖方向（只允许向下）：

```
cmd/agent → internal/agent → { internal/task, internal/event, internal/prompt,
                               internal/tools, domain/llm }
internal/input → internal/event        （产生 userMessage 事件）
internal/task  → internal/event        （存储事件）
```

---

## 6. 一次任务的端到端时序（示例）

任务："统计本目录 Go 代码行数，写进 report.md"

```
input.Source(stdin) ──"统计...写进 report.md"──► userMessage 事件 ─► Store(task_42)
                                                                        │ 触发
迭代1: Assemble(task_42) → 渲染提示词 → LLM
       → tool_call: plan_update([列文件, 统计, 写报告], current=1)
       → PlanManager 拦截 → plan_update 事件 + 合成 observation("计划已建立")
                                                                        │ 触发
迭代2: LLM → tool_call: terminal(find . -name '*.go' | xargs wc -l)
       → action_start 事件(Sink 显示"$ find ...")
       → 沙箱执行 → observation 事件(行数输出)
                                                                        │ 触发
迭代3: LLM → tool_call: plan_advance(2 完成) → plan_update 事件 + 合成 observation
                                                                        │ 触发
迭代4: LLM → tool_call: file_write(report.md, ...)
       → action_start → 执行 → observation("写入成功")
                                                                        │ 触发
迭代5: LLM → 纯文本"已完成，报告在 report.md"
       → assistant_message 事件 + task_done(完成)
       → Terminator 收尾，等待下一条 userMessage

全程：每个事件产生的同时推给 event.Sink → 终端实时滚动显示
```

---

## 7. 设计决策记录（为什么这么设计）

1. **事件流而非消息数组。** 消息数组只能服务"喂给 LLM"一个目的；事件流同时服务
   上下文、展示、持久化、恢复四个目的，且 task_id 重放即恢复。这是参考工程 D 层
   最值得萃取的一条结构性思想。
2. **每轮强制单工具。** 多工具并发会让"观察→再决策"的因果链变模糊，错误归因困难。
   单工具让循环成为严格的状态机，代价（多几轮迭代）对终端场景可接受。
3. **修复链做成 LLM 调用的中间件。** 鲁棒性逻辑（重复检测、非法工具、坏参数）
   与循环主干解耦，每个修复器可独立增删测。
4. **计划用元工具而非独立 planner agent。** 单二进制目标下双 agent 太重；
   元工具方案让"修订计划"与"调用工具"同构，模型用同一套机制学会两件事。
5. **Reflection 不设独立反思阶段。** 反思拆进三处：错误 observation 回注 +
   error_handling 提示词段（轻量自纠）、repetition 修复器（原地打转检测）、
   plan_update 携带的状态变化（计划级反思）。每轮多调一次"反思 LLM"对
   终端场景延迟/成本不划算。
6. **Source/Sink/Store 三接口收口外界。** E/F 层的全部价值对本项目而言只是
   "输入进得来、过程看得见、历史存得住"，三个接口足矣，实现可以无限简单。

---

## 8. 实施路线（对应 status.md 的 Roadmap）

1. **M1 事件流打底**：event/task 包从壳变实，重写 Loop 为事件驱动（迁移现有
   function-calling 逻辑），ConsoleSink 直出过程。
2. **M2 上下文组装**：Assembler 实现事件→消息映射，多轮对话跑通（同 task_id 续聊）。
3. **M3 REPL + 流式**：input.Source 升级为交互循环，接通 B 层已有流式能力。
4. **M4 提示词装配**：PromptRenderer 按段渲染，接入 templates/。
5. **M5 计划层**：PlanManager + 三个元工具 + planner.md 段。
6. **M6 修复链**：按 §4.4 表格逐个实现修复器。
7. **M7 滑动窗口**：Assembler 补压缩；**M8 持久化 Store**：换文件/SQLite，得 resume。
