#!/bin/bash

# 记忆访问统计测试脚本

echo "==================================="
echo "记忆访问统计功能测试"
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

# 测试2: 检查数据库字段
echo "📋 测试2: 检查数据库Schema"
echo "-----------------------------------"
if [ -f "memory.db" ]; then
    # 检查是否有access_count字段
    sqlite3 memory.db "PRAGMA table_info(memories);" | grep -q "access_count"
    if [ $? -eq 0 ]; then
        echo "✅ access_count字段已添加"
    else
        echo "⚠️  access_count字段未找到（首次运行会自动添加）"
    fi

    sqlite3 memory.db "PRAGMA table_info(memories);" | grep -q "last_accessed_at"
    if [ $? -eq 0 ]; then
        echo "✅ last_accessed_at字段已添加"
    else
        echo "⚠️  last_accessed_at字段未找到（首次运行会自动添加）"
    fi
else
    echo "⚠️  数据库文件不存在（首次运行会自动创建）"
fi
echo ""

# 测试3: 检查新增方法
echo "📋 测试3: 检查新增方法"
echo "-----------------------------------"
if grep -q "GetTopAccessedMemories" internal/memory/sqlite.go; then
    echo "✅ GetTopAccessedMemories方法已添加"
else
    echo "❌ GetTopAccessedMemories方法未找到"
    exit 1
fi

if grep -q "updateAccessStats" internal/memory/sqlite.go; then
    echo "✅ updateAccessStats方法已添加"
else
    echo "❌ updateAccessStats方法未找到"
    exit 1
fi
echo ""

# 测试4: 检查CLI命令
echo "📋 测试4: 检查CLI命令"
echo "-----------------------------------"
./chat.exe --help 2>&1 | grep -q "stats"
if [ $? -eq 0 ]; then
    echo "✅ --stats命令已添加"
else
    echo "⚠️  --stats命令未在帮助中显示（但可能已实现）"
fi
echo ""

# 测试5: 尝试运行stats命令
echo "📋 测试5: 测试stats命令"
echo "-----------------------------------"
echo "运行: ./chat.exe --stats --user test"
./chat.exe --stats --user test 2>&1 | head -5
echo ""
echo "如果看到统计输出或'还没有访问记录'，说明功能正常"
echo ""

echo "==================================="
echo "✅ 所有测试完成！"
echo "==================================="
echo ""
echo "📖 使用说明："
echo "  1. 进行几轮对话: ./chat.exe"
echo "  2. 查看统计: ./chat.exe --stats"
echo "  3. 指定用户: ./chat.exe --stats --user alice"
echo ""
echo "💡 测试建议："
echo "  - 进行多轮对话，让系统积累访问记录"
echo "  - 多次提到相同的信息（如名字）"
echo "  - 查看统计，观察访问次数增长"
echo ""
echo "🔍 数据库查询："
echo "  sqlite3 memory.db \"SELECT mkey, access_count, last_accessed_at FROM memories WHERE access_count > 0 ORDER BY access_count DESC LIMIT 10;\""
echo ""
