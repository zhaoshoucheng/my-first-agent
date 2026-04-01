# 架构文档

## 项目概述

这是一个基于 Go 语言的智能体(Agent)框架，借鉴了 LangChain 和 ReAct 等流行框架的设计理念。

## 核心概念

### 1. Agent (智能体)
智能体是系统的核心，负责协调 LLM、工具和记忆系统，完成用户的任务。

### 2. LLM (大语言模型)
与 LLM 服务提供商（OpenAI、Anthropic 等）进行交互的客户端。

### 3. Tools (工具)
智能体可以调用的外部工具，如计算器、搜索引擎等。

### 4. Memory (记忆)
存储对话历史和上下文信息。

## 架构图

```
┌─────────────────────────────────────────────────────────┐
│                         Agent                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │     LLM     │  │   Memory    │  │    Tools    │    │
│  │   Client    │  │   System    │  │  Registry   │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
│                                                          │
│  ┌─────────────────────────────────────────────────┐   │
│  │              Executor (执行器)                   │   │
│  │  - ReAct 模式                                    │   │
│  │  - Plan-and-Execute 模式                        │   │
│  └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## 目录结构

```
my-first-agent/
├── cmd/                    # 应用程序入口
│   └── agent/             # 主程序
├── internal/              # 内部包（不对外暴露）
│   ├── agent/            # 智能体核心逻辑
│   ├── llm/              # LLM 客户端
│   ├── memory/           # 记忆系统
│   ├── tools/            # 工具实现
│   └── prompt/           # 提示词模板
├── pkg/                   # 公共包（可对外暴露）
│   └── types/            # 类型定义
├── config/               # 配置文件
├── examples/             # 示例代码
└── docs/                 # 文档
```

## 设计模式

### ReAct 模式
Reasoning and Acting 模式，智能体通过以下循环解决问题：

1. **Thought**: 思考下一步做什么
2. **Action**: 选择一个工具执行
3. **Action Input**: 提供工具输入
4. **Observation**: 观察工具执行结果
5. 重复 1-4 直到得出最终答案

### 工具注册模式
所有工具都实现 `Tool` 接口，并在 `Registry` 中注册，便于管理和扩展。

### 记忆抽象
通过 `Memory` 接口抽象不同的记忆实现：
- Buffer Memory: 简单的缓冲记忆
- Summary Memory: 摘要记忆
- Vector Memory: 向量数据库记忆

## 扩展指南

### 添加新工具

1. 在 `internal/tools/` 创建新文件
2. 实现 `types.Tool` 接口：
   ```go
   type MyTool struct{}

   func (t *MyTool) Name() string { ... }
   func (t *MyTool) Description() string { ... }
   func (t *MyTool) Execute(ctx context.Context, input string) (string, error) { ... }
   ```
3. 在主程序中注册工具

### 添加新的 LLM 提供商

1. 在 `internal/llm/types.go` 添加新的 Provider 常量
2. 在 `internal/llm/client.go` 实现对应的客户端逻辑

### 实现新的记忆类型

1. 在 `internal/memory/` 创建新文件
2. 实现 `types.Memory` 接口

## 待实现功能

- [ ] LLM 客户端实际调用逻辑
- [ ] ReAct 执行器完整实现
- [ ] 搜索工具实现
- [ ] 向量记忆实现
- [ ] 更多工具类型
- [ ] 流式输出支持
- [ ] 异步工具执行
- [ ] 多智能体协作
