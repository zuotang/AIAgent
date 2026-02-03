#!/bin/bash

echo "=== 记忆触发方法测试 ==="
echo ""

# 测试用例
test_cases=(
    "以后叫我左"
    "好的"
    "我喜欢编程"
    "今天天气不错"
    "我的目标是学习AI"
    "哈哈"
    "Python是我的主力语言"
    "左，这是我的名字"
)

echo "测试用例："
for i in "${!test_cases[@]}"; do
    echo "$((i+1)). ${test_cases[$i]}"
done
echo ""

echo "三种触发方法的对比："
echo ""
echo "1. keyword（关键词）- 快速但可能遗漏"
echo "2. llm（LLM分类器）- 准确但需要额外API调用"
echo "3. conservative（保守策略）- 简单可靠，信任提取器"
echo ""

echo "配置方法："
echo "编辑 config.deepseek.yaml 中的 trigger_method:"
echo "  - keyword: 基于关键词匹配"
echo "  - llm: 使用小模型分类（需要 Ollama 运行 qwen2.5:0.5b）"
echo "  - conservative: 基于消息长度（推荐）"
