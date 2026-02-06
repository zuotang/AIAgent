package registry

import (
	"context"
	"fmt"
	"sync"

	"agent-langchain/internal/workflow/types"
)

// NodeRunner 节点运行器接口
type NodeRunner interface {
	// Run 执行节点逻辑
	// ctx: 上下文
	// rc: 运行时上下文（包含依赖）
	// inputs: 输入端口数据
	// params: 节点参数
	// 返回: 输出端口数据和错误
	Run(ctx context.Context, rc *RunContext, inputs map[string]any, params map[string]any) (map[string]any, error)
}

// NodeSpec 节点规范
type NodeSpec struct {
	Type    string            `json:"type"`    // 节点类型（如 "LLM.Generate"）
	Version string            `json:"version"` // 节点版本
	Inputs  []types.PortSpec  `json:"inputs"`  // 输入端口规范
	Outputs []types.PortSpec  `json:"outputs"` // 输出端口规范
	Runner  NodeRunner        `json:"-"`       // 节点运行器（不序列化）
}

// RunContext 运行时上下文，包含所有外部依赖
type RunContext struct {
	// LLM 客户端
	LLMClient interface {
		Chat(ctx context.Context, msgs []any, model ...string) (string, error)
	}

	// Embedding 客户端
	EmbedClient interface {
		Embed(ctx context.Context, text string) ([]float32, error)
	}

	// Memory 存储
	MemoryStore interface {
		// 这里定义 Memory 接口方法
		// TODO: 根据实际 memory.Store 接口定义
	}

	// Qdrant 客户端
	QdrantClient interface {
		// 这里定义 Qdrant 接口方法
		// TODO: 根据实际 qdrant.Store 接口定义
	}

	// Tool 注册表
	ToolRegistry interface {
		// 这里定义 Tool 接口方法
		// TODO: 根据实际 tools 接口定义
	}

	// 缓存（可选）
	Cache map[string]any
}

// Registry 节点注册中心
type Registry struct {
	mu    sync.RWMutex
	specs map[string]*NodeSpec // key: "type@version"
}

// NewRegistry 创建新的注册中心
func NewRegistry() *Registry {
	return &Registry{
		specs: make(map[string]*NodeSpec),
	}
}

// Register 注册节点规范
func (r *Registry) Register(spec *NodeSpec) error {
	if spec.Type == "" {
		return fmt.Errorf("node type is required")
	}
	if spec.Version == "" {
		return fmt.Errorf("node version is required")
	}
	if spec.Runner == nil {
		return fmt.Errorf("node runner is required")
	}

	key := makeKey(spec.Type, spec.Version)

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.specs[key]; exists {
		return fmt.Errorf("node %s@%s already registered", spec.Type, spec.Version)
	}

	r.specs[key] = spec
	return nil
}

// Get 获取节点规范
func (r *Registry) Get(nodeType, version string) (*NodeSpec, error) {
	key := makeKey(nodeType, version)

	r.mu.RLock()
	defer r.mu.RUnlock()

	spec, ok := r.specs[key]
	if !ok {
		return nil, fmt.Errorf("node %s@%s not found", nodeType, version)
	}

	return spec, nil
}

// List 列出所有注册的节点
func (r *Registry) List() []*NodeSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	specs := make([]*NodeSpec, 0, len(r.specs))
	for _, spec := range r.specs {
		specs = append(specs, spec)
	}
	return specs
}

// makeKey 生成注册键
func makeKey(nodeType, version string) string {
	return fmt.Sprintf("%s@%s", nodeType, version)
}

// DefaultRegistry 全局默认注册中心
var DefaultRegistry = NewRegistry()
