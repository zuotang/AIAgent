#!/bin/bash

# 工作流API测试脚本

API_URL="http://localhost:8080"

echo "=== 工作流API测试 ==="
echo ""

# 1. 测试健康检查
echo "1. 测试健康检查..."
curl -s "${API_URL}/health" | jq .
echo ""

# 2. 获取可用节点
echo "2. 获取可用节点..."
curl -s "${API_URL}/api/workflow/nodes" | jq '.nodes | length'
echo "节点数量: $(curl -s "${API_URL}/api/workflow/nodes" | jq '.nodes | length')"
echo ""

# 3. 获取节点详情（前3个）
echo "3. 节点列表（前3个）:"
curl -s "${API_URL}/api/workflow/nodes" | jq '.nodes[0:3] | .[] | {type, category, name}'
echo ""

# 4. 校验简单工作流
echo "4. 校验工作流..."
cat > /tmp/test_workflow.json << 'EOF'
{
  "version": "1.0",
  "meta": {
    "id": "wf-test",
    "name": "测试工作流"
  },
  "nodes": {
    "input": {
      "id": "input",
      "type": "Input.Text",
      "version": "1.0",
      "params": {
        "text": "Hello"
      }
    },
    "transform": {
      "id": "transform",
      "type": "Transform.TextToMessages",
      "version": "1.0",
      "params": {
        "role": "user"
      }
    }
  },
  "edges": [
    {
      "id": "e1",
      "from": { "node": "input", "port": "text" },
      "to": { "node": "transform", "port": "text" },
      "type": "data"
    }
  ]
}
EOF

curl -s -X POST "${API_URL}/api/workflow/validate" \
  -H "Content-Type: application/json" \
  -d @/tmp/test_workflow.json | jq .
echo ""

# 5. 执行简单工作流
echo "5. 执行工作流..."
cat > /tmp/execute_workflow.json << 'EOF'
{
  "workflow": {
    "version": "1.0",
    "meta": {
      "id": "wf-execute-test",
      "name": "执行测试"
    },
    "nodes": {
      "input": {
        "id": "input",
        "type": "Input.Text",
        "version": "1.0",
        "params": {
          "text": "测试文本"
        }
      },
      "output": {
        "id": "output",
        "type": "Output.Text",
        "version": "1.0",
        "params": {}
      }
    },
    "edges": [
      {
        "id": "e1",
        "from": { "node": "input", "port": "text" },
        "to": { "node": "output", "port": "text" },
        "type": "data"
      }
    ]
  },
  "async": false
}
EOF

curl -s -X POST "${API_URL}/api/workflow/execute" \
  -H "Content-Type: application/json" \
  -d @/tmp/execute_workflow.json | jq '.success, .trace.status'
echo ""

echo "=== 测试完成 ==="
