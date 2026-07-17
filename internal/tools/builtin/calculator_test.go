package builtin

import (
	"context"
	"testing"
)

func TestCalculator_Execute(t *testing.T) {
	calc := NewCalculator()
	ctx := context.Background()

	cases := []struct {
		name string
		expr string
		want string
	}{
		{"integer add", "1 + 2", "3"},
		{"precedence", "(25 * 4) + 10", "110"},
		{"mul over add", "2 + 3 * 4", "14"},
		{"division", "10 / 4", "2.5"},
		{"unary minus", "-5 + 3", "-2"},
		{"unary plus", "+7", "7"},
		{"nested parens", "((1 + 2) * (3 + 4))", "21"},
		{"decimals", "0.1 + 0.2", "0.30000000000000004"},
		{"whitespace tolerant", "  6  *  7  ", "42"},
		{"double negative", "--3", "3"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := calc.Execute(ctx, map[string]any{"expression": tc.expr})
			if err != nil {
				t.Fatalf("Execute(%q): %v", tc.expr, err)
			}
			if got != tc.want {
				t.Errorf("Execute(%q) = %q, want %q", tc.expr, got, tc.want)
			}
		})
	}
}

func TestCalculator_Errors(t *testing.T) {
	calc := NewCalculator()
	ctx := context.Background()

	cases := []struct {
		name string
		expr string
	}{
		{"division by zero", "1 / 0"},
		{"missing closing paren", "(1 + 2"},
		{"trailing operator", "1 +"},
		{"empty expression", ""},
		{"unexpected token", "1 $ 2"},
		{"bare letters", "abc"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := calc.Execute(ctx, map[string]any{"expression": tc.expr})
			if err == nil {
				t.Errorf("Execute(%q) expected error, got nil", tc.expr)
			}
		})
	}
}

func TestCalculator_Metadata(t *testing.T) {
	calc := NewCalculator()
	if calc.Name() != "calculator" {
		t.Errorf("Name() = %q", calc.Name())
	}
	if calc.Description() == "" {
		t.Error("Description() is empty")
	}
	// 必填项 expression 应出现在 schema 的 required 中。
	schema := calc.Parameters()
	req, _ := schema["required"].([]any)
	found := false
	for _, r := range req {
		if r == "expression" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'expression' in required, schema=%v", schema)
	}
}
