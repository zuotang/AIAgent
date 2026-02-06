package types

// PortType 定义端口类型
type PortType string

// 端口类型常量
const (
	// 基础类型
	PortTypeMessages PortType = "messages"     // LLM 消息列表
	PortTypeText     PortType = "text"         // 纯文本
	PortTypeJSON     PortType = "json"         // JSON 数据

	// 向量相关
	PortTypeEmbedding     PortType = "embedding"      // 向量嵌入
	PortTypeVectorQuery   PortType = "vector_query"   // 向量查询请求
	PortTypeVectorResults PortType = "vector_results" // 向量查询结果

	// 记忆和知识库
	PortTypeMemoryItems PortType = "memory_items" // 记忆项列表
	PortTypeKBDocs      PortType = "kb_docs"      // 知识库文档

	// 工具相关
	PortTypeToolCall   PortType = "tool_call"   // 工具调用请求
	PortTypeToolResult PortType = "tool_result" // 工具执行结果

	// 上下文和配置
	PortTypeContextPack PortType = "context_pack" // 打包的上下文
	PortTypeLLMConfig   PortType = "llm_config"   // LLM 配置

	// 控制流
	PortTypeFlow PortType = "flow" // 控制流信号
)

// PortSpec 定义端口规范
type PortSpec struct {
	Name     string   `json:"name"`     // 端口名称
	Type     PortType `json:"type"`     // 端口类型
	Required bool     `json:"required"` // 是否必需
}

// NodeStatus 节点执行状态
type NodeStatus string

const (
	NodeStatusPending  NodeStatus = "pending"  // 等待执行
	NodeStatusRunning  NodeStatus = "running"  // 正在执行
	NodeStatusSuccess  NodeStatus = "success"  // 执行成功
	NodeStatusError    NodeStatus = "error"    // 执行失败
	NodeStatusSkipped  NodeStatus = "skipped"  // 跳过执行
)
