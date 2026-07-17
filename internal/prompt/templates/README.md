# 系统提示词分段模板

萃取自参考工程 `agent-server` 提示词层的**结构思想**（分段拼装、模块化职责），
文字为本项目自行重写，以 Alembic 自己的名义编写，不含任何外部产品信息。

## 拼装顺序

PromptRenderer（待实现，见 `docs/architecture/agent-core.md` §4.2）按以下顺序拼接：

| 顺序 | 文件 | 职责 | 何时装配 |
|---|---|---|---|
| 1 | `system.md` | 身份与能力总述 | 总是 |
| 2 | `agent_loop.md` | 循环行为契约（每轮单工具等） | 总是 |
| 3 | `planner.md` | 计划模块使用规则 | 总是（计划层做好后） |
| 4 | `todo.md` | todo.md 文件维护规则 | 有 file 工具时 |
| 5 | `error_handling.md` | 工具失败时的自纠策略 | 总是 |

## 约定

- 模板是 Go `text/template` 语法，变量：`{{.Language}}`（回复语言）、
  `{{.WorkDir}}`（工作目录）。段与段之间用空行分隔拼接。
- 提示词正文用英文（跨模型遵循度最好），用户可见输出语言由 `{{.Language}}` 控制。
- 每段用 XML 风格标签包裹（`<agent_loop>`…），便于模型区分段落边界。
