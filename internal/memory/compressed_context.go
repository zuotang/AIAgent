package memory

import (
	"context"
	"time"
)

// CompressedContext 压缩后的上下文
type CompressedContext struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID          string    `gorm:"type:text;not null" json:"user_id"`
	AgentID         uint      `gorm:"type:integer;not null;default:1" json:"agent_id"` // Agent ID，用于隔离不同 agent 的上下文
	CompressedText  string    `gorm:"type:text" json:"compressed_text"`           // 压缩后的文本
	LastMessageID   uint      `gorm:"type:integer" json:"last_message_id"`        // 最后处理的消息ID
	LastCompressAt  time.Time `gorm:"autoUpdateTime" json:"last_compress_at"`     // 最后压缩时间
	UncompressedLen int       `gorm:"type:integer" json:"uncompressed_len"`       // 未压缩部分长度
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (CompressedContext) TableName() string {
	return "compressed_contexts"
}

// GetCompressedContext 获取用户的压缩上下文
func (s *Store) GetCompressedContext(ctx context.Context, userID string, agentID uint) (*CompressedContext, error) {
	var cc CompressedContext
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ?", userID, agentID).
		First(&cc).Error

	if err != nil {
		return nil, err
	}
	return &cc, nil
}

// UpsertCompressedContext 更新或创建压缩上下文
func (s *Store) UpsertCompressedContext(ctx context.Context, cc *CompressedContext) error {
	var existing CompressedContext
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ?", cc.UserID, cc.AgentID).
		First(&existing).Error

	if err != nil {
		// 不存在，创建新的
		return s.db.WithContext(ctx).Create(cc).Error
	}

	// 存在，更新
	return s.db.WithContext(ctx).
		Model(&existing).
		Updates(map[string]interface{}{
			"compressed_text":  cc.CompressedText,
			"last_message_id":  cc.LastMessageID,
			"uncompressed_len": cc.UncompressedLen,
		}).Error
}

// ClearCompressedContext 清除用户的压缩上下文
func (s *Store) ClearCompressedContext(ctx context.Context, userID string, agentID uint) error {
	return s.db.WithContext(ctx).
		Where("user_id = ? AND agent_id = ?", userID, agentID).
		Delete(&CompressedContext{}).Error
}
