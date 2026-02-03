#!/bin/bash

# 计算器工具测试脚本

echo "==================================="
echo "计算器工具功能测试"
echo "==================================="
echo ""

# 测试1: 单元测试
echo "📋 测试1: 运行单元测试"
echo "-----------------------------------"
cd "$(dirname "$0")"
go test ./internal/tools/... -v
echo ""

# 测试2: 编译检查
echo "📋 测试2: 编译主程序"
echo "-----------------------------------"
go build -o chat.exe ./cmd/chat
if [ $? -eq 0 ]; then
    echo "✅ 编译成功"
else
    echo "❌ 编译失败"
    exit 1
fi
echo ""

# 测试3: 直接测试计算器
echo "📋 测试3: 直接测试计算器功能"
echo "-----------------------------------"
cat > /tmp/test_calculator.go << 'EOF'
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
        "10-4",
        "5*6",
        "20/4",
        "2^10",
        "sqrt(16)",
        "(2+3)*4",
    }

    fmt.Println("计算器测试结果：")
    for _, expr := range tests {
        result, err := calc.Execute(ctx, expr)
        if err != nil {
            fmt.Printf("  %s = ERROR: %v\n", expr, err)
        } else {
            fmt.Printf("  %s = %s\n", expr, result)
        }
    }
}
EOF

go run /tmp/test_calculator.go
echo ""

# 测试4: 工具调用提取测试
echo "📋 测试4: 测试工具调用提取"
echo "-----------------------------------"
cat > /tmp/test_extract.go << 'EOF'
package main

import (
    "fmt"
    "strings"
)

type ToolCall struct {
    ToolName  string
    Arguments string
}

func extractToolCall(response string) *ToolCall {
    idx := strings.Index(response, "TOOL_CALL:")
    if idx == -1 {
        return nil
    }

    callPart := strings.TrimSpace(response[idx+len("TOOL_CALL:"):])
    openParen := strings.Index(callPart, "(")
    if openParen == -1 {
        return nil
    }

    toolName := strings.TrimSpace(callPart[:openParen])
    closeParen := strings.LastIndex(callPart, ")")
    if closeParen == -1 || closeParen <= openParen {
        return nil
    }

    args := callPart[openParen+1 : closeParen]
    args = strings.Trim(args, `"'`)

    return &ToolCall{
        ToolName:  toolName,
        Arguments: args,
    }
}

func main() {
    tests := []string{
        `TOOL_CALL: calculator("2+3")`,
        `让我算一下 TOOL_CALL: calculator("2^10")`,
        `TOOL_CALL: calculator("sqrt(16)")`,
        `普通文本，没有工具调用`,
    }

    fmt.Println("工具调用提取测试：")
    for i, test := range tests {
        result := extractToolCall(test)
        if result != nil {
            fmt.Printf("  测试%d: ✅ 提取成功 - %s(%s)\n", i+1, result.ToolName, result.Arguments)
        } else {
            fmt.Printf("  测试%d: ⚪ 无工具调用\n", i+1)
        }
    }
}
EOF

go run /tmp/test_extract.go
echo ""

echo "==================================="
echo "✅ 所有测试完成！"
echo "==================================="
echo ""
echo "📖 使用说明："
echo "  1. 启动程序: ./chat.exe"
echo "  2. 输入UID（或直接回车使用默认）"
echo "  3. 尝试问数学问题："
echo "     - '2的10次方是多少？'"
echo "     - '帮我算一下 (5+3)*4'"
echo "     - '16的平方根'"
echo ""
echo "🔍 调试模式："
echo "  在 config.yaml 中设置 debug: true"
echo "  可以看到工具调用的详细过程"
echo ""
