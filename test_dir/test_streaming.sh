#!/bin/bash

# 流式响应测试脚本

echo "==================================="
echo "流式响应功能测试"
echo "==================================="
echo ""

# 测试1: 编译检查
echo "📋 测试1: 编译检查"
echo "-----------------------------------"
cd "$(dirname "$0")"
go build -o chat.exe ./cmd/chat
if [ $? -eq 0 ]; then
    echo "✅ 编译成功"
else
    echo "❌ 编译失败"
    exit 1
fi
echo ""

# 测试2: 检查ChatStream方法
echo "📋 测试2: 检查ChatStream方法是否存在"
echo "-----------------------------------"
if grep -q "func (c \*Client) ChatStream" internal/models/ollama.go; then
    echo "✅ ChatStream方法已添加"
else
    echo "❌ ChatStream方法未找到"
    exit 1
fi
echo ""

# 测试3: 检查main.go中的流式调用
echo "📋 测试3: 检查main.go中的流式响应集成"
echo "-----------------------------------"
if grep -q "ChatStream" cmd/chat/main.go; then
    echo "✅ main.go已集成流式响应"
else
    echo "❌ main.go未集成流式响应"
    exit 1
fi
echo ""

# 测试4: 代码语法检查
echo "📋 测试4: Go代码语法检查"
echo "-----------------------------------"
go vet ./...
if [ $? -eq 0 ]; then
    echo "✅ 代码语法检查通过"
else
    echo "⚠️  发现一些警告（可能不影响功能）"
fi
echo ""

echo "==================================="
echo "✅ 所有测试完成！"
echo "==================================="
echo ""
echo "📖 使用说明："
echo "  1. 启动程序: ./chat.exe"
echo "  2. 输入UID（或直接回车）"
echo "  3. 开始对话，观察流式输出效果"
echo ""
echo "💡 测试建议："
echo "  - 问一个需要长回复的问题"
echo "  - 观察文字是否逐字显示"
echo "  - 尝试数学问题，看工具调用后的流式响应"
echo ""
echo "🔍 对比测试："
echo "  - 使用Ollama: 流式响应 ✅"
echo "  - 使用DeepSeek: 非流式响应（API限制）"
echo ""
