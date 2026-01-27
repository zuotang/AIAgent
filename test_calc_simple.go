package main

import (
    "context"
    "fmt"
    "agent-langchain/internal/tools"
)

func main() {
    calc := &tools.CalculatorTool{}
    ctx := context.Background()

    tests := []string{
        "2+3",
        "2^10",
        "sqrt(16)",
        "(2+3)*4",
    }

    fmt.Println("计算器功能验证：")
    for _, expr := range tests {
        result, err := calc.Execute(ctx, expr)
        if err != nil {
            fmt.Printf("  ❌ %s = ERROR: %v\n", expr, err)
        } else {
            fmt.Printf("  ✅ %s = %s\n", expr, result)
        }
    }
}
