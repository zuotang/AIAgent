#!/bin/bash

echo "测试流式 API 响应..."
echo "================================"

curl -N -X POST http://localhost:8080/api/chat/stream \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test_user",
    "message": "你好，请简单介绍一下你自己",
    "agent_id": 1
  }'

echo ""
echo "================================"
echo "测试完成"
