package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	// 导入 modernc.org/sqlite 驱动（纯 Go 实现，无需 CGO）
	_ "modernc.org/sqlite"
)

type Store struct {
	db *gorm.DB
}

// New 创建新的 Store 实例
func New(dbPath string) (*Store, error) {
	// 首先使用 database/sql 打开连接（使用 modernc.org/sqlite）
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 配置 GORM
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // 生产环境使用 Silent
	}

	// 使用已有的 sql.DB 创建 GORM 实例
	db, err := gorm.Open(sqlite.Dialector{
		Conn: sqlDB,
	}, config)
	if err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize GORM: %w", err)
	}

	s := &Store{db: db}

	// 自动迁移表结构
	if err := s.init(); err != nil {
		return nil, err
	}

	return s, nil
}

// Close 关闭数据库连接
func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// init 初始化数据库表
func (s *Store) init() error {
	// 检查表是否存在
	var tableCount int64
	s.db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='memories'").Scan(&tableCount)

	if tableCount == 0 {
		// 表不存在，手动创建
		ddl := `
CREATE TABLE IF NOT EXISTS memories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id TEXT NOT NULL,
  agent_id INTEGER NOT NULL DEFAULT 1,
  mtype TEXT NOT NULL,
  mkey  TEXT NOT NULL,
  mvalue TEXT NOT NULL,
  confidence REAL NOT NULL DEFAULT 0.7,
  owner TEXT NOT NULL DEFAULT 'user',
  layer INTEGER NOT NULL DEFAULT 2,
  importance REAL NOT NULL DEFAULT 0.5,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  access_count INTEGER NOT NULL DEFAULT 0,
  last_accessed_at DATETIME,
  deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS chat_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id TEXT NOT NULL,
  agent_id INTEGER NOT NULL DEFAULT 1,
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  session_id TEXT
);

CREATE TABLE IF NOT EXISTS prompts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  content TEXT NOT NULL,
  description TEXT,
  category TEXT DEFAULT 'assistant',
  is_default BOOLEAN DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS agents (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  gender TEXT,
  age INTEGER,
  description TEXT,
  prompt_id INTEGER NOT NULL,
  avatar TEXT,
  config TEXT,
  is_active BOOLEAN DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at DATETIME,
  FOREIGN KEY (prompt_id) REFERENCES prompts(id)
);

CREATE TABLE IF NOT EXISTS compressed_contexts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id TEXT NOT NULL,
  agent_id INTEGER NOT NULL DEFAULT 1,
  compressed_text TEXT,
  last_message_id INTEGER,
  uncompressed_len INTEGER DEFAULT 0,
  last_compress_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_memories_unique ON memories(user_id, agent_id, mtype, mkey, owner);
CREATE INDEX IF NOT EXISTS idx_chat_history_user_id ON chat_history(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_history_session_id ON chat_history(session_id);
CREATE INDEX IF NOT EXISTS idx_memories_deleted_at ON memories(deleted_at);
CREATE INDEX IF NOT EXISTS idx_compressed_contexts_unique ON compressed_contexts(user_id, agent_id);
CREATE INDEX IF NOT EXISTS idx_prompts_category ON prompts(category);
CREATE INDEX IF NOT EXISTS idx_agents_prompt_id ON agents(prompt_id);
CREATE INDEX IF NOT EXISTS idx_agents_is_active ON agents(is_active);
`
		if err := s.db.Exec(ddl).Error; err != nil {
			return fmt.Errorf("failed to create tables: %w", err)
		}

		// 插入默认提示词
		if err := s.insertDefaultPrompts(); err != nil {
			return fmt.Errorf("failed to insert default prompts: %w", err)
		}
	} else {
		// 表已存在，添加可能缺失的列（兼容旧版本）
		s.db.Exec(`ALTER TABLE memories ADD COLUMN access_count INTEGER NOT NULL DEFAULT 0`)
		s.db.Exec(`ALTER TABLE memories ADD COLUMN last_accessed_at DATETIME`)
		s.db.Exec(`ALTER TABLE memories ADD COLUMN deleted_at DATETIME`)
		s.db.Exec(`ALTER TABLE memories ADD COLUMN agent_id INTEGER NOT NULL DEFAULT 1`)
		s.db.Exec(`ALTER TABLE chat_history ADD COLUMN agent_id INTEGER NOT NULL DEFAULT 1`)
		s.db.Exec(`ALTER TABLE compressed_contexts ADD COLUMN agent_id INTEGER NOT NULL DEFAULT 1`)

		// 确保索引存在
		s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_memories_unique ON memories(user_id, agent_id, mtype, mkey, owner)`)
		s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_history_user_id ON chat_history(user_id)`)
		s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_history_session_id ON chat_history(session_id)`)
		s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_memories_deleted_at ON memories(deleted_at)`)
		s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_compressed_contexts_unique ON compressed_contexts(user_id, agent_id)`)

		// 检查并创建新表
		var promptTableCount int64
		s.db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='prompts'").Scan(&promptTableCount)
		if promptTableCount == 0 {
			s.db.Exec(`
CREATE TABLE IF NOT EXISTS prompts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  content TEXT NOT NULL,
  description TEXT,
  category TEXT DEFAULT 'assistant',
  is_default BOOLEAN DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_prompts_category ON prompts(category);
`)
			s.insertDefaultPrompts()
		}

		var agentTableCount int64
		s.db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='agents'").Scan(&agentTableCount)
		if agentTableCount == 0 {
			s.db.Exec(`
CREATE TABLE IF NOT EXISTS agents (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  gender TEXT,
  age INTEGER,
  description TEXT,
  prompt_id INTEGER NOT NULL,
  avatar TEXT,
  config TEXT,
  is_active BOOLEAN DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at DATETIME,
  FOREIGN KEY (prompt_id) REFERENCES prompts(id)
);
CREATE INDEX IF NOT EXISTS idx_agents_prompt_id ON agents(prompt_id);
CREATE INDEX IF NOT EXISTS idx_agents_is_active ON agents(is_active);
`)
		}
	}

	return nil
}

// insertDefaultPrompts 插入默认提示词
func (s *Store) insertDefaultPrompts() error {
	defaultPrompts := []Prompt{
		{
			Name:        "默认助手",
			Content:     "你是一个友好、专业的 AI 助手。你的目标是帮助用户解决问题，提供准确的信息和建议。",
			Description: "通用 AI 助手提示词",
			Category:    "assistant",
			IsDefault:   true,
		},
		{
			Name:        "翻译助手",
			Content:     "你是一个专业的翻译助手。请准确、流畅地翻译用户提供的文本，保持原文的语气和风格。",
			Description: "专业翻译提示词",
			Category:    "translator",
			IsDefault:   false,
		},
		{
			Name:        "代码助手",
			Content:     "你是一个专业的编程助手。你精通多种编程语言，能够帮助用户编写、调试和优化代码。请提供清晰的代码示例和解释。",
			Description: "编程辅助提示词",
			Category:    "coder",
			IsDefault:   false,
		},
	}

	for _, prompt := range defaultPrompts {
		// 检查是否已存在
		var count int64
		s.db.Model(&Prompt{}).Where("name = ?", prompt.Name).Count(&count)
		if count == 0 {
			if err := s.db.Create(&prompt).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

// SetDebug 设置调试模式
func (s *Store) SetDebug(debug bool) {
	if debug {
		s.db.Logger = logger.Default.LogMode(logger.Info)
	} else {
		s.db.Logger = logger.Default.LogMode(logger.Silent)
	}
}

// SaveChatMessage 保存聊天消息并返回消息ID
func (s *Store) SaveChatMessage(ctx context.Context, userID, role, content, sessionID string, agentID uint) (uint, error) {
	msg := &ChatMessage{
		UserID:    userID,
		AgentID:   agentID,
		Role:      role,
		Content:   content,
		SessionID: sessionID,
	}

	err := s.db.WithContext(ctx).Create(msg).Error
	if err != nil {
		return 0, err
	}
	return msg.ID, nil
}

// GetChatHistory 获取用户的聊天记录（基于偏移量的分页，保留用于兼容性）
func (s *Store) GetChatHistory(ctx context.Context, userID string, agentID uint, limit, offset int) ([]ChatMessage, error) {
	var messages []ChatMessage

	err := s.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ?", userID, agentID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages).Error

	return messages, err
}

// GetChatHistoryWithCursor 获取用户的聊天记录（基于游标的分页）
// beforeID: 获取此ID之前的消息（用于向前翻页），0表示从最新消息开始
// limit: 每页返回的消息数量
// 返回消息按ID倒序排列（最新的在前）
func (s *Store) GetChatHistoryWithCursor(ctx context.Context, userID string, agentID uint, beforeID uint, limit int) ([]ChatMessage, error) {
	var messages []ChatMessage
	query := s.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ?", userID, agentID)

	// 如果指定了 beforeID，只获取 ID 小于该值的消息
	if beforeID > 0 {
		query = query.Where("id < ?", beforeID)
	}

	err := query.
		Order("id DESC").
		Limit(limit).
		//需要将数据反转，因为数据库查询是倒序的

		Find(&messages).Error

	return messages, err
}

// GetChatHistoryCount 获取聊天记录总数
func (s *Store) GetChatHistoryCount(ctx context.Context, userID string, agentID uint) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&ChatMessage{}).
		Where("user_id = ? AND agent_id = ?", userID, agentID).
		Count(&count).Error
	return count, err
}

// GetChatHistoryAfterID 获取指定消息ID之后的聊天记录
func (s *Store) GetChatHistoryAfterID(ctx context.Context, userID string, agentID uint, afterID uint, limit int) ([]ChatMessage, error) {
	var messages []ChatMessage

	err := s.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ? AND id > ?", userID, agentID, afterID).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error

	return messages, err
}

// GetChatHistoryBetweenIDs 获取指定区间内的聊天记录（按ID升序）
// afterID: 只返回ID > afterID
// beforeID: 只返回ID < beforeID（beforeID=0表示不限制上界）
func (s *Store) GetChatHistoryBetweenIDs(ctx context.Context, userID string, agentID uint, afterID uint, beforeID uint, limit int) ([]ChatMessage, error) {
	var messages []ChatMessage

	query := s.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ? AND id > ?", userID, agentID, afterID)
	if beforeID > 0 {
		query = query.Where("id < ?", beforeID)
	}

	err := query.
		Order("id ASC").
		Limit(limit).
		Find(&messages).Error

	return messages, err
}

// GetChatSessions 获取用户的聊天会话列表
func (s *Store) GetChatSessions(ctx context.Context, userID string, agentID uint) ([]ChatSession, error) {
	var sessions []ChatSession

	err := s.db.WithContext(ctx).
		Model(&ChatMessage{}).
		Select("session_id, MAX(content) as latest_message, MAX(created_at) as last_activity").
		Where("user_id = ? AND agent_id = ? AND session_id != ''", userID, agentID).
		Group("session_id").
		Order("last_activity DESC").
		Scan(&sessions).Error

	return sessions, err
}

// UpsertExtractedMemories 插入或更新提取的记忆
func (s *Store) UpsertExtractedMemories(ctx context.Context, userID string, agentID uint, memories []ExtractedMemory) error {
	if len(memories) == 0 {
		return nil
	}

	// 使用事务批量处理
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		for _, m := range memories {
			memory := &Memory{
				UserID:     userID,
				AgentID:    agentID,
				Type:       m.Type,
				Key:        m.Key,
				Value:      m.Value,
				Confidence: m.Confidence,
				Owner:      m.Owner,
				UpdatedAt:  now,
			}

			// 使用 GORM 的 Clauses 来处理冲突
			// 如果记录已存在（基于唯一约束），则更新 value, confidence, updated_at
			err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "user_id"},
					{Name: "agent_id"},
					{Name: "mtype"},
					{Name: "mkey"},
					{Name: "owner"},
				},
				DoUpdates: clause.AssignmentColumns([]string{"mvalue", "confidence", "updated_at"}),
			}).Create(memory).Error

			if err != nil {
				return err
			}
		}

		return nil
	})
}

// RenderStructuredMemory 渲染结构化记忆为文本
func (s *Store) RenderStructuredMemory(ctx context.Context, userID string, agentID uint, limit int) (string, error) {
	var memories []Memory

	err := s.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ?", userID, agentID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&memories).Error

	if err != nil {
		return "", err
	}

	if len(memories) == 0 {
		return "(暂无长期记忆)", nil
	}

	// 按 owner 分组
	userMems := []Memory{}
	agentMems := []Memory{}

	for _, m := range memories {
		if m.Owner == "user" {
			userMems = append(userMems, m)
		} else {
			agentMems = append(agentMems, m)
		}
	}

	var sb strings.Builder

	// 用户记忆
	if len(userMems) > 0 {
		sb.WriteString("【用户信息】\n")
		for _, m := range userMems {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", m.Key, m.Value))
		}
	}

	// Agent 记忆
	if len(agentMems) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("【助手记忆】\n")
		for _, m := range agentMems {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", m.Key, m.Value))
		}
	}

	return sb.String(), nil
}

// GetTopAccessedMemories 获取访问次数最多的记忆
func (s *Store) GetTopAccessedMemories(ctx context.Context, userID string, agentID uint, limit int) ([]MemoryStat, error) {
	var stats []MemoryStat

	err := s.db.WithContext(ctx).
		Model(&Memory{}).
		Select("owner, mtype as type, mkey as key, mvalue as value, confidence, access_count, "+
			"datetime(last_accessed_at) as last_accessed, datetime(updated_at) as updated_at").
		Where("user_id = ? AND agent_id = ? AND access_count > 0", userID, agentID).
		Order("access_count DESC, updated_at DESC").
		Limit(limit).
		Scan(&stats).Error

	return stats, err
}

// IncrementAccessCount 增加记忆的访问次数
func (s *Store) IncrementAccessCount(ctx context.Context, userID string, agentID uint, mtype, mkey, owner string) error {
	now := time.Now()

	return s.db.WithContext(ctx).
		Model(&Memory{}).
		Where("user_id = ? AND agent_id = ? AND mtype = ? AND mkey = ? AND owner = ?", userID, agentID, mtype, mkey, owner).
		Updates(map[string]interface{}{
			"access_count":     gorm.Expr("access_count + 1"),
			"last_accessed_at": now,
		}).Error
}

// LoadProfile 加载配置文件（保留兼容性）
func (s *Store) LoadProfile(profileType, key string) (*Profile, error) {
	// 这个方法在当前代码中似乎没有被使用，保留空实现以保持兼容性
	return &Profile{
		Agent: make(map[string]any),
		User:  make(map[string]any),
	}, nil
}

// SaveProfile 保存配置文件（保留兼容性）
func (s *Store) SaveProfile(profile *Profile) error {
	// 这个方法在当前代码中似乎没有被使用，保留空实现以保持兼容性
	return nil
}

// GetMemoriesByType 根据类型获取记忆
func (s *Store) GetMemoriesByType(ctx context.Context, userID string, agentID uint, mtype string) ([]Memory, error) {
	var memories []Memory

	err := s.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ? AND mtype = ?", userID, agentID, mtype).
		Order("updated_at DESC").
		Find(&memories).Error

	return memories, err
}

// DeleteMemory 删除记忆（软删除）
func (s *Store) DeleteMemory(ctx context.Context, userID string, agentID uint, mtype, mkey, owner string) error {
	return s.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ? AND mtype = ? AND mkey = ? AND owner = ?", userID, agentID, mtype, mkey, owner).
		Delete(&Memory{}).Error
}

// GetMemoryCount 获取记忆总数
func (s *Store) GetMemoryCount(ctx context.Context, userID string, agentID uint) (int64, error) {
	var count int64

	err := s.db.WithContext(ctx).
		Model(&Memory{}).
		Where("user_id = ? AND agent_id = ?", userID, agentID).
		Count(&count).Error

	return count, err
}

// ExportMemories 导出所有记忆为 JSON
func (s *Store) ExportMemories(ctx context.Context, userID string, agentID uint) (string, error) {
	var memories []Memory

	err := s.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ?", userID, agentID).
		Order("updated_at DESC").
		Find(&memories).Error

	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(memories, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// ==================== Prompt CRUD ====================

// CreatePrompt 创建提示词
func (s *Store) CreatePrompt(ctx context.Context, prompt *Prompt) error {
	return s.db.WithContext(ctx).Create(prompt).Error
}

// GetPrompt 获取单个提示词
func (s *Store) GetPrompt(ctx context.Context, id uint) (*Prompt, error) {
	var prompt Prompt
	err := s.db.WithContext(ctx).First(&prompt, id).Error
	if err != nil {
		return nil, err
	}
	return &prompt, nil
}

// GetPromptByName 根据名称获取提示词
func (s *Store) GetPromptByName(ctx context.Context, name string) (*Prompt, error) {
	var prompt Prompt
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&prompt).Error
	if err != nil {
		return nil, err
	}
	return &prompt, nil
}

// ListPrompts 列出所有提示词
func (s *Store) ListPrompts(ctx context.Context, category string, limit, offset int) ([]Prompt, error) {
	var prompts []Prompt
	query := s.db.WithContext(ctx)

	if category != "" {
		query = query.Where("category = ?", category)
	}

	err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&prompts).Error

	return prompts, err
}

// UpdatePrompt 更新提示词
func (s *Store) UpdatePrompt(ctx context.Context, id uint, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).
		Model(&Prompt{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// DeletePrompt 删除提示词（软删除）
func (s *Store) DeletePrompt(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&Prompt{}, id).Error
}

// GetDefaultPrompt 获取默认提示词
func (s *Store) GetDefaultPrompt(ctx context.Context) (*Prompt, error) {
	var prompt Prompt
	err := s.db.WithContext(ctx).Where("is_default = ?", true).First(&prompt).Error
	if err != nil {
		return nil, err
	}
	return &prompt, nil
}

// ==================== Agent CRUD ====================

// CreateAgent 创建 Agent
func (s *Store) CreateAgent(ctx context.Context, agent *Agent) error {
	return s.db.WithContext(ctx).Create(agent).Error
}

// GetAgent 获取单个 Agent
func (s *Store) GetAgent(ctx context.Context, id uint) (*Agent, error) {
	var agent Agent
	err := s.db.WithContext(ctx).Preload("Prompt").First(&agent, id).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// GetAgentByName 根据名称获取 Agent
func (s *Store) GetAgentByName(ctx context.Context, name string) (*Agent, error) {
	var agent Agent
	err := s.db.WithContext(ctx).Preload("Prompt").Where("name = ?", name).First(&agent).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// ListAgents 列出所有 Agent
func (s *Store) ListAgents(ctx context.Context, isActive *bool, limit, offset int) ([]Agent, error) {
	var agents []Agent
	query := s.db.WithContext(ctx).Preload("Prompt")

	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&agents).Error

	return agents, err
}

// UpdateAgent 更新 Agent
func (s *Store) UpdateAgent(ctx context.Context, id uint, updates map[string]interface{}) error {
	return s.db.WithContext(ctx).
		Model(&Agent{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// DeleteAgent 删除 Agent（软删除）
func (s *Store) DeleteAgent(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&Agent{}, id).Error
}

// GetActiveAgents 获取所有激活的 Agent
func (s *Store) GetActiveAgents(ctx context.Context) ([]Agent, error) {
	var agents []Agent
	err := s.db.WithContext(ctx).
		Preload("Prompt").
		Where("is_active = ?", true).
		Order("created_at DESC").
		Find(&agents).Error
	return agents, err
}

// ==================== Clear Data ====================

// ClearChatHistory 清空指定用户和 Agent 的聊天记录
func (s *Store) ClearChatHistory(ctx context.Context, userID string, agentID uint) error {
	return s.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ?", userID, agentID).
		Delete(&ChatMessage{}).Error
}

// ClearMemories 清空指定用户和 Agent 的记忆
func (s *Store) ClearMemories(ctx context.Context, userID string, agentID uint) error {
	return s.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ?", userID, agentID).
		Delete(&Memory{}).Error
}

// ClearAllData 清空指定用户和 Agent 的所有数据（聊天记录、记忆、压缩上下文）
func (s *Store) ClearAllData(ctx context.Context, userID string, agentID uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 清空聊天记录
		if err := tx.Where("user_id = ? AND agent_id = ?", userID, agentID).Delete(&ChatMessage{}).Error; err != nil {
			return err
		}
		// 清空记忆
		if err := tx.Where("user_id = ? AND agent_id = ?", userID, agentID).Delete(&Memory{}).Error; err != nil {
			return err
		}
		// 清空压缩上下文
		if err := tx.Where("user_id = ? AND agent_id = ?", userID, agentID).Delete(&CompressedContext{}).Error; err != nil {
			return err
		}
		return nil
	})
}
