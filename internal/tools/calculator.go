package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Calculator 计算器工具
type Calculator struct{}

// NewCalculator 创建计算器工具
func NewCalculator() *Calculator {
	return &Calculator{}
}

// Name 返回工具名称
func (c *Calculator) Name() string {
	return "calculator"
}

// Description 返回工具描述
func (c *Calculator) Description() string {
	return "A calculator tool for performing basic arithmetic operations. " +
		"Input should be a mathematical expression like '2 + 2' or '10 * 5'."
}

// Execute 执行计算
func (c *Calculator) Execute(ctx context.Context, input string) (string, error) {
	// TODO: 实现更完整的数学表达式解析
	// 这里只是一个简单的示例
	input = strings.TrimSpace(input)

	// 简单的示例：只处理基本的两个数字运算
	parts := strings.Fields(input)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid input format, expected: 'number operator number'")
	}

	num1, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return "", fmt.Errorf("invalid first number: %v", err)
	}

	num2, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return "", fmt.Errorf("invalid second number: %v", err)
	}

	var result float64
	switch parts[1] {
	case "+":
		result = num1 + num2
	case "-":
		result = num1 - num2
	case "*":
		result = num1 * num2
	case "/":
		if num2 == 0 {
			return "", fmt.Errorf("division by zero")
		}
		result = num1 / num2
	default:
		return "", fmt.Errorf("unsupported operator: %s", parts[1])
	}

	return fmt.Sprintf("%.2f", result), nil
}
