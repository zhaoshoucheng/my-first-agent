# My First Agent

一个基于 Go 语言的智能体（Agent）框架，借鉴了 LangChain 和 ReAct 等流行框架的设计理念。

## 项目特性

- 🤖 **模块化设计**: 清晰的架构，易于理解和扩展
- 🔧 **工具系统**: 可插拔的工具注册机制
- 🧠 **记忆管理**: 支持多种记忆类型（Buffer、Summary、Vector）
- 🎯 **ReAct 模式**: 实现了 Reasoning and Acting 智能体模式
- 🔌 **多 LLM 支持**: 支持 OpenAI、Anthropic 等多个 LLM 提供商

## 项目结构

```
my-first-agent/
├── cmd/                    # 应用程序入口
│   └── agent/             # 主程序
├── internal/              # 内部包
│   ├── agent/            # 智能体核心逻辑
│   ├── llm/              # LLM 客户端
│   ├── memory/           # 记忆系统
│   ├── tools/            # 工具实现
│   └── prompt/           # 提示词模板
├── pkg/                   # 公共包
│   └── types/            # 类型定义
├── config/               # 配置文件
├── examples/             # 示例代码
└── docs/                 # 文档
```

## 快速开始

### 1. 环境准备

确保已安装 Go 1.22 或更高版本：

```bash
go version
```

### 2. 配置环境变量

复制环境变量示例文件并填入你的 API 密钥：

```bash
cp .env.example .env
# 编辑 .env 文件，填入你的 API 密钥
```

### 3. 安装依赖

```bash
make install
```

### 4. 运行示例

```bash
make example
```

## 核心概念

### Agent (智能体)
智能体是系统的核心，负责协调 LLM、工具和记忆系统，完成用户的任务。

### LLM (大语言模型)
与 LLM 服务提供商（OpenAI、Anthropic 等）进行交互的客户端。

### Tools (工具)
智能体可以调用的外部工具，如：
- **Calculator**: 数学计算工具
- **Search**: 网络搜索工具（待实现）

### Memory (记忆)
存储对话历史和上下文信息：
- **Buffer Memory**: 保存最近的 N 条消息
- **Summary Memory**: 对历史消息进行摘要（待实现）
- **Vector Memory**: 使用向量数据库存储和检索（待实现）

## 使用示例

```go
package main

import (
    "context"
    "log"

    "github.com/shoucheng/my-first-agent/internal/agent"
    "github.com/shoucheng/my-first-agent/internal/llm"
    "github.com/shoucheng/my-first-agent/internal/memory"
    "github.com/shoucheng/my-first-agent/internal/tools"
    "github.com/shoucheng/my-first-agent/pkg/types"
)

func main() {
    ctx := context.Background()

    // 创建 LLM 客户端
    llmClient, _ := llm.NewClient(llm.Config{
        Provider: llm.ProviderOpenAI,
        APIKey:   "your-api-key",
        Model:    "gpt-4",
    })

    // 创建记忆和工具
    mem := memory.NewBufferMemory(10)
    toolRegistry := tools.NewRegistry()
    toolRegistry.Register(tools.NewCalculator())

    // 创建智能体
    myAgent, _ := agent.New(llmClient, mem, toolRegistry, types.AgentConfig{
        MaxIterations: 10,
        Verbose:       true,
    })

    // 运行
    response, _ := myAgent.Run(ctx, "计算 25 * 4 + 10")
    log.Println(response)
}
```

## 开发计划

- [ ] 实现 LLM 客户端的实际调用逻辑
- [ ] 完成 ReAct 执行器
- [ ] 实现搜索工具
- [ ] 添加向量记忆支持
- [ ] 支持流式输出
- [ ] 添加更多工具类型
- [ ] 实现 Plan-and-Execute 模式
- [ ] 多智能体协作

## 学习资源

- [LangChain 文档](https://python.langchain.com/)
- [ReAct 论文](https://arxiv.org/abs/2210.03629)
- [智能体设计模式](docs/architecture.md)

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可

MIT License