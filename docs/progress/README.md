# 项目进展文档（progress）

**项目名：Alembic** —— *distill the essence of every agent*（萃取每一个 agent 的精华）。
名字取自炼金术里的蒸馏器（alembic），呼应"萃取本质"的纲领。

这是一个**探索性学习项目**：用 Go 萃取各种参考工程（如 `agent-server`）的核心思路，
做一个能"**吸收、提炼别的 agent、取长补短**"的本地终端 agent——从别人身上萃取的不是
代码，而是本质。

每次打开 Claude，先读这套文档就能知道：项目是什么、参考工程怎么拆模块、现在做到哪了、
下一步可能做什么、有哪些不可违反的规则。

## 文档导航

- **[principles.md](./principles.md)** — 纲领：只萃取 agent 的**本质复杂度**，砍掉 E/F。
  方向准绳，跑偏时 Claude 须提醒。**每次必读。**
- **[rules.md](./rules.md)** — 两条硬性红线（禁品牌字眼、禁硬编码密钥）。**每次必读。**
- **[module-map.md](./module-map.md)** — 参考工程 `agent-server` 的 6 层模块地图，啃的顺序。
- **[status.md](./status.md)** — 当前进展对照表 + Roadmap（最常更新的文件）。
- **[../architecture/agent-core.md](../architecture/agent-core.md)** — 本工程自身的目标架构：
  事件流为中心的 Agent 核心设计（主循环逐步拆解、模块/包布局、设计决策、实施路线）。

## 工作方式

1. 模块很大，先看 `module-map.md` 选定要做的层。
2. 要看参考工程某模块的源码细节时，去 `.claude/local-notes.md`（不入库）里的速查表
   找对应位置，再按需深入。
3. 做完一个模块，更新 `status.md`，并同步顶层 `CLAUDE.md` 的 Architecture 段。
