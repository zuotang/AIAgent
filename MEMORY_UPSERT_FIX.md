# Memory Upsert 错误修复方案

## 问题描述

当前的 `UpsertExtractedMemories` 方法在处理重复记忆时会触发 UNIQUE 约束错误：

```
UNIQUE constraint failed: memories.user_id, memories.agent_id, memories.mtype, memories.mkey, memories.owner
```

## 根本原因

`FirstOrCreate` + `Assign` 的组合不能正确处理 UPSERT 逻辑：
- `FirstOrCreate` 只在记录不存在时创建
- `Assign` 在 `FirstOrCreate` 中不会触发更新
- 导致重复插入时违反 UNIQUE 约束

## 修复方案

### 方案1：使用 GORM Clauses（推荐）

```go
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

			// 使用 Clauses 实现真正的 UPSERT
			result := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "user_id"},
					{Name: "agent_id"},
					{Name: "mtype"},
					{Name: "mkey"},
					{Name: "owner"},
				},
				DoUpdates: clause.AssignmentColumns([]string{
					"mvalue",
					"confidence",
					"updated_at",
				}),
			}).Create(memory)

			if result.Error != nil {
				return result.Error
			}
		}

		return nil
	})
}
```

**需要添加的导入**：
```go
import (
	"gorm.io/gorm/clause"
)
```

### 方案2：先查询再更新（更安全但性能稍低）

```go
// UpsertExtractedMemories 插入或更新提取的记忆
func (s *Store) UpsertExtractedMemories(ctx context.Context, userID string, agentID uint, memories []ExtractedMemory) error {
	if len(memories) == 0 {
		return nil
	}

	// 使用事务批量处理
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		for _, m := range memories {
			// 先查询是否存在
			var existing Memory
			err := tx.Where("user_id = ? AND agent_id = ? AND mtype = ? AND mkey = ? AND owner = ?",
				userID, agentID, m.Type, m.Key, m.Owner).
				First(&existing).Error

			if err == gorm.ErrRecordNotFound {
				// 不存在，创建新记录
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
				if err := tx.Create(memory).Error; err != nil {
					return err
				}
			} else if err != nil {
				// 查询出错
				return err
			} else {
				// 存在，更新记录
				if err := tx.Model(&existing).Updates(map[string]interface{}{
					"mvalue":     m.Value,
					"confidence": m.Confidence,
					"updated_at": now,
				}).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}
```

## 推荐方案

**方案1（使用 Clauses）** 更优，因为：
1. 性能更好（单次数据库操作）
2. 原子性更强（数据库级别的 UPSERT）
3. 代码更简洁

## 实施步骤

1. 修改 `internal/memory/sqlite.go` 中的 `UpsertExtractedMemories` 方法
2. 添加必要的导入 `gorm.io/gorm/clause`
3. 重新编译并测试
4. 验证不再出现 UNIQUE 约束错误

## 测试验证

```bash
# 重新编译
go build -o main.exe ./cmd/api

# 运行并观察日志
./main.exe -config config.yaml

# 发送包含重复信息的对话，验证不再报错
```
