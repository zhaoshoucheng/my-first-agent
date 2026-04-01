# Gemini 流式函数调用测试

这个示例展示如何使用 Gemini API 进行流式函数调用（Stream Function Calling）。

## 功能特点

- ✅ 流式输出（实时返回响应）
- ✅ 函数调用（Function Calling / Tool Use）
- ✅ 多个工具定义（天气查询 + 计算器）
- ✅ OpenAI 类似的格式

## 安装依赖

```bash
go get github.com/google/generative-ai-go/genai
go get google.golang.org/api/option
```

## 配置

1. 获取 Gemini API Key：https://makersuite.google.com/app/apikey

2. 设置环境变量：
```bash
export GEMINI_API_KEY="your-api-key-here"
```

或者添加到 `.env` 文件：
```bash
GEMINI_API_KEY=your-api-key-here
```

## 运行

```bash
cd examples/gemini-stream
go run main.go
```

## 示例输出

```
用户: 北京的天气怎么样？另外帮我计算 25 * 4 + 10

AI 响应 (流式输出):
---
[函数调用] get_current_weather
  参数: map[location:北京]
[函数调用] calculator
  参数: map[expression:25 * 4 + 10]
---

执行函数调用...
  - get_current_weather: {"location": "北京", "temperature": "22", "unit": "celsius", "condition": "晴朗"}
  - calculator: {"expression": "25 * 4 + 10", "result": "110"}

获取最终答案 (流式输出):
---
北京目前天气晴朗，气温22摄氏度。25 * 4 + 10 的计算结果是 110。
---

测试完成！
```

## 与 OpenAI 格式对比

### OpenAI Function Calling
```json
{
  "model": "gpt-4",
  "messages": [...],
  "functions": [
    {
      "name": "get_current_weather",
      "description": "获取天气",
      "parameters": {
        "type": "object",
        "properties": {
          "location": {"type": "string"}
        }
      }
    }
  ],
  "stream": true
}
```

### Gemini Function Calling
```go
&genai.FunctionDeclaration{
    Name:        "get_current_weather",
    Description: "获取天气",
    Parameters: &genai.Schema{
        Type: genai.TypeObject,
        Properties: map[string]*genai.Schema{
            "location": {Type: genai.TypeString},
        },
    },
}
```

## 核心概念

1. **FunctionDeclaration**: 定义可调用的函数（工具）
2. **SendMessageStream**: 发送消息并获取流式响应
3. **FunctionCall**: AI 决定调用的函数及参数
4. **FunctionResponse**: 函数执行结果返回给 AI
5. **迭代处理**: 通过 `iter.Next()` 逐块接收响应

## 扩展建议

- [ ] 添加真实的天气 API 调用
- [ ] 实现真正的计算器逻辑
- [ ] 添加更多工具（搜索、数据库查询等）
- [ ] 实现错误处理和重试机制
- [ ] 添加对话历史管理
