package memory

import (
	"time"

	"gorm.io/gorm"
)

// Profile 是给 system 注入用的长期设定（你之前的 agent/user 结构）
type Profile struct {
	Agent map[string]any `json:"agent"`
	User  map[string]any `json:"user"`
}

// ExtractedMemory 提取的记忆（用于 JSON 传输）
type ExtractedMemory struct {
	Type       string  `json:"type"`        // profile | preference | rule | fact
	Key        string  `json:"key"`         // e.g. "language"
	Value      string  `json:"value"`       // e.g. "Chinese"
	Confidence float64 `json:"confidence"`  // 0~1
	AlsoVector bool    `json:"also_vector"` // 是否也写入 Qdrant（便于检索）
	Text       string  `json:"text"`        // 写入 Qdrant 的语义文本（可选）
	Owner      string  `json:"owner"`       // user | agent，标识记忆的所有者
}

// Memory GORM 模型 - 数据库中的记忆表
type Memory struct {
	ID             uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         string         `gorm:"type:text;not null" json:"user_id"`
	Type           string         `gorm:"column:mtype;type:text;not null" json:"type"`
	Key            string         `gorm:"column:mkey;type:text;not null" json:"key"`
	Value          string         `gorm:"column:mvalue;type:text;not null" json:"value"`
	Confidence     float64        `gorm:"type:real;not null;default:0.7" json:"confidence"`
	Owner          string         `gorm:"type:text;not null;default:'user'" json:"owner"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	AccessCount    int            `gorm:"type:integer;not null;default:0" json:"access_count"`
	LastAccessedAt *time.Time     `gorm:"type:datetime" json:"last_accessed_at,omitempty"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Memory) TableName() string {
	return "memories"
}

// ChatMessage 聊天消息 GORM 模型
type ChatMessage struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    string    `gorm:"type:text;not null" json:"user_id"`
	Role      string    `gorm:"type:text;not null" json:"role"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	SessionID string    `gorm:"type:text" json:"session_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (ChatMessage) TableName() string {
	return "chat_history"
}

// ChatSession 聊天会话
type ChatSession struct {
	SessionID     string    `json:"session_id"`
	LatestMessage string    `json:"latest_message"`
	LastActivity  time.Time `json:"last_activity"`
}

// MemoryStat 记忆访问统计
type MemoryStat struct {
	Owner        string    `json:"owner"`
	Type         string    `json:"type"`
	Key          string    `json:"key"`
	Value        string    `json:"value"`
	Confidence   float64   `json:"confidence"`
	AccessCount  int       `json:"access_count"`
	LastAccessed string    `json:"last_accessed"`
	UpdatedAt    string    `json:"updated_at"`
}

// Prompt 系统提示词模型
type Prompt struct {
	ID          uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string         `gorm:"type:text;not null;uniqueIndex" json:"name"`
	Content     string         `gorm:"type:text;not null" json:"content"`
	Description string         `gorm:"type:text" json:"description"`
	Category    string         `gorm:"type:text;default:'assistant'" json:"category"` // assistant, translator, coder, etc.
	IsDefault   bool           `gorm:"type:boolean;default:false" json:"is_default"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Prompt) TableName() string {
	return "prompts"
}

// Agent Agent 配置模型
type Agent struct {
	ID          uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string         `gorm:"type:text;not null;uniqueIndex" json:"name"`
	Gender      string         `gorm:"type:text" json:"gender"`                       // 性别
	Age         int            `gorm:"type:integer" json:"age"`                       // 年龄
	Description string         `gorm:"type:text" json:"description"`
	PromptID    uint           `gorm:"type:integer;not null" json:"prompt_id"`
	Prompt      *Prompt        `gorm:"foreignKey:PromptID" json:"prompt,omitempty"` // 关联的提示词
	Avatar      string         `gorm:"type:text" json:"avatar"`                      // 头像 URL
	Config      string         `gorm:"type:text" json:"config"`                      // JSON 配置（temperature, model 等）
	IsActive    bool           `gorm:"type:boolean;default:true" json:"is_active"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Agent) TableName() string {
	return "agents"
}

// AgentConfig Agent 配置结构（用于解析 Config JSON）
type AgentConfig struct {
	Temperature float64 `json:"temperature,omitempty"`
	Model       string  `json:"model,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}
