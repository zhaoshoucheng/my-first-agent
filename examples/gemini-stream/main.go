package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

func main() {
	ctx := context.Background()

	// 从环境变量获取 API Key
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("请设置 GEMINI_API_KEY 环境变量")
	}

	// 创建客户端
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	// 使用 Gemini 1.5 Pro
	model := client.GenerativeModel("gemini-1.5-pro")

	// 定义函数（工具）- 类似 OpenAI 的 function calling
	getCurrentWeather := &genai.FunctionDeclaration{
		Name:        "get_current_weather",
		Description: "获取指定城市的当前天气",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"location": {
					Type:        genai.TypeString,
					Description: "城市名称，例如：北京、上海",
				},
				"unit": {
					Type:        genai.TypeString,
					Description: "温度单位",
					Enum:        []string{"celsius", "fahrenheit"},
				},
			},
			Required: []string{"location"},
		},
	}

	// 定义计算器函数
	calculator := &genai.FunctionDeclaration{
		Name:        "calculator",
		Description: "执行数学计算",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"expression": {
					Type:        genai.TypeString,
					Description: "数学表达式，例如：2 + 2, 10 * 5",
				},
			},
			Required: []string{"expression"},
		},
	}

	// 配置工具
	model.Tools = []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				getCurrentWeather,
				calculator,
			},
		},
	}

	// 开始对话
	session := model.StartChat()

	// 用户问题
	question := "北京的天气怎么样？另外帮我计算 25 * 4 + 10"
	fmt.Printf("用户: %s\n\n", question)

	// 发送消息并获取流式响应
	fmt.Println("AI 响应 (流式输出):")
	fmt.Println("---")

	iter := session.SendMessageStream(ctx, genai.Text(question))

	var functionCalls []*genai.FunctionCall

	// 处理流式响应
	for {
		resp, err := iter.Next()
		if err != nil {
			if err.Error() == "iterator done" {
				break
			}
			log.Fatalf("获取响应失败: %v", err)
		}

		// 打印每个 chunk
		for _, candidate := range resp.Candidates {
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					// 处理文本部分
					if text, ok := part.(genai.Text); ok {
						fmt.Print(string(text))
					}

					// 处理函数调用
					if fc, ok := part.(genai.FunctionCall); ok {
						fmt.Printf("\n[函数调用] %s\n", fc.Name)
						fmt.Printf("  参数: %v\n", fc.Args)
						functionCalls = append(functionCalls, &fc)
					}
				}
			}
		}
	}

	fmt.Println("\n---")

	// 如果有函数调用，模拟执行并返回结果
	if len(functionCalls) > 0 {
		fmt.Println("\n执行函数调用...")

		var functionResponses []genai.Part

		for _, fc := range functionCalls {
			var result string

			switch fc.Name {
			case "get_current_weather":
				location := fc.Args["location"].(string)
				result = fmt.Sprintf(`{"location": "%s", "temperature": "22", "unit": "celsius", "condition": "晴朗"}`, location)

			case "calculator":
				expression := fc.Args["expression"].(string)
				// 简化处理，实际应该真正计算
				result = fmt.Sprintf(`{"expression": "%s", "result": "110"}`, expression)

			default:
				result = `{"error": "未知函数"}`
			}

			fmt.Printf("  - %s: %s\n", fc.Name, result)

			functionResponses = append(functionResponses, genai.FunctionResponse{
				Name: fc.Name,
				Response: map[string]any{
					"content": result,
				},
			})
		}

		// 将函数结果返回给模型，获取最终答案
		fmt.Println("\n获取最终答案 (流式输出):")
		fmt.Println("---")

		iter2 := session.SendMessageStream(ctx, functionResponses...)

		for {
			resp, err := iter2.Next()
			if err != nil {
				if err.Error() == "iterator done" {
					break
				}
				log.Fatalf("获取最终响应失败: %v", err)
			}

			for _, candidate := range resp.Candidates {
				if candidate.Content != nil {
					for _, part := range candidate.Content.Parts {
						if text, ok := part.(genai.Text); ok {
							fmt.Print(string(text))
						}
					}
				}
			}
		}

		fmt.Println("\n---")
	}

	fmt.Println("\n测试完成！")
}
