# ORM 迁移说明

## 更新日期
2026-01-31

## 概述

本次更新将项目的数据库访问层从原生 SQL 迁移到 GORM（Go 最流行的 ORM 框架），提供更好的类型安全、代码可维护性和开发效率。

## 主要变更

### 1. 依赖变更

**新增依赖**：
```go
gorm.io/gorm v1.31.1
gorm.io/driver/sqlite v1.6.0
```

**保留依赖**：
- `modernc.org/sqlite` - GORM SQLite 驱动内部使用

### 2. 数据模型定义

#### Memory 模型（记忆表）

```go
type Memory struct {
    ID             uint           `gorm:"primaryKey;autoIncrement"`
    UserID         string         `gorm:"type:text;not null;index:idx_user_type_key,priority:1"`
    Type           string         `gorm:"column:mtype;type:text;not null;index:idx_user_type_key,priority:2"`
    Key            string         `gorm:"column:mkey;type:text;not null;index:idx_user_type_key,priority:3"`
    Value          string         `gorm:"column:mvalue;type:text;not null"`
    Confidence     float64        `gorm:"type:real;not null;default:0.7"`
    Owner          string         `gorm:"type:text;not null;default:'user';index:idx_user_type_key,priority:4"`
    UpdatedAt      time.Time      `gorm:"autoUpdateTime"`
    AccessCount    int            `gorm:"type:integer;not null;default:0"`
    LastAccessedAt *time.Time     `gorm:"type:datetime"`
    DeletedAt      gorm.DeletedAt `gorm:"index"`
}
```

**特性**：
- 自动主键管理
- 复合索引支持
- 软删除支持（DeletedAt）
- 自动时间戳（UpdatedAt）
- 类型安全的字段定义

#### ChatMessage 模型（聊天消息表）

```go
type ChatMessage struct {
    ID        uint      `gorm:"primaryKey;autoIncrement"`
    UserID    string    `gorm:"type:text;not null;index:idx_chat_user"`
    Role      string    `gorm:"type:text;not null"`
    Content   string    `gorm:"type:text;not null"`
    SessionID string    `gorm:"type:text;index:idx_chat_session"`
    CreatedAt time.Time `gorm:"autoCreateTime"`
}
```

**特性**：
- 自动创建时间戳
- 索引优化查询
- 类型安全

### 3. Store 结构变更

**变更前**：
```go
type Store struct {
    db *sql.DB
}
```

**变更后**：
```go
type Store struct {
    db *gorm.DB
}
```

### 4. 数据库初始化

**自动迁移**：
```go
func (s *Store) init() error {
    // GORM 自动迁移表结构
    if err := s.db.AutoMigrate(&Memory{}, &ChatMessage{}); err != nil {
        return fmt.Errorf("failed to migrate database: %w", err)
    }

    // 创建复合唯一索引
    if err := s.db.Exec(`
        CREATE UNIQUE INDEX IF NOT EXISTS idx_memories_unique
        ON memories(user_id, mtype, mkey, owner)
    `).Error; err != nil {
        return fmt.Errorf("failed to create unique index: %w", err)
    }

    return nil
}
```

**优势**：
- 自动创建表和索引
- 自动添加新字段（向后兼容）
- 无需手动编写 DDL

### 5. 方法重写对照

#### SaveChatMessage

**变更前**（原生 SQL）：
```go
func (s *Store) SaveChatMessage(ctx context.Context, userID, role, content, sessionID string) error {
    _, err := s.db.ExecContext(ctx, `
        INSERT INTO chat_history (user_id, role, content, session_id)
        VALUES (?, ?, ?, ?)
    `, userID, role, content, sessionID)
    return err
}
```

**变更后**（GORM）：
```go
func (s *Store) SaveChatMessage(ctx context.Context, userID, role, content, sessionID string) error {
    msg := &ChatMessage{
        UserID:    userID,
        Role:      role,
        Content:   content,
        SessionID: sessionID,
    }
    return s.db.WithContext(ctx).Create(msg).Error
}
```

**优势**：
- 类型安全
- 自动处理时间戳
- 更清晰的代码

#### GetChatHistory

**变更前**（原生 SQL）：
```go
func (s *Store) GetChatHistory(ctx context.Context, userID string, limit, offset int) ([]ChatMessage, error) {
    rows, err := s.db.QueryContext(ctx, `
        SELECT id, role, content, session_id, created_at
        FROM chat_history
        WHERE user_id = ?
        ORDER BY created_at DESC
        LIMIT ? OFFSET ?
    `, userID, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var messages []ChatMessage
    for rows.Next() {
        var msg ChatMessage
        if err := rows.Scan(&msg.ID, &msg.Role, &msg.Content, &msg.SessionID, &msg.CreatedAt); err != nil {
            return nil, err
        }
        messages = append(messages, msg)
    }
    return messages, rows.Err()
}
```

**变更后**（GORM）：
```go
func (s *Store) GetChatHistory(ctx context.Context, userID string, limit, offset int) ([]ChatMessage, error) {
    var messages []ChatMessage

    err := s.db.WithContext(ctx).
        Where("user_id = ?", userID).
        Order("created_at DESC").
        Limit(limit).
        Offset(offset).
        Find(&messages).Error

    return messages, err
}
```

**优势**：
- 链式调用，更易读
- 自动处理结果映射
- 无需手动管理 rows

#### UpsertExtractedMemories

**变更前**（原生 SQL）：
```go
func (s *Store) UpsertExtractedMemories(ctx context.Context, userID string, memories []ExtractedMemory) error {
    // 复杂的 SQL UPSERT 逻辑
    for _, m := range memories {
        _, err := s.db.ExecContext(ctx, `
            INSERT INTO memories (user_id, mtype, mkey, mvalue, confidence, owner, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(user_id, mtype, mkey, owner)
            DO UPDATE SET mvalue=excluded.mvalue, confidence=excluded.confidence, updated_at=excluded.updated_at
        `, userID, m.Type, m.Key, m.Value, m.Confidence, m.Owner, time.Now())
        if err != nil {
            return err
        }
    }
    return nil
}
```

**变更后**（GORM）：
```go
func (s *Store) UpsertExtractedMemories(ctx context.Context, userID string, memories []ExtractedMemory) error {
    return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        now := time.Now()

        for _, m := range memories {
            memory := &Memory{
                UserID:     userID,
                Type:       m.Type,
                Key:        m.Key,
                Value:      m.Value,
                Confidence: m.Confidence,
                Owner:      m.Owner,
                UpdatedAt:  now,
            }

            result := tx.Where("user_id = ? AND mtype = ? AND mkey = ? AND owner = ?",
                userID, m.Type, m.Key, m.Owner).
                Assign(map[string]interface{}{
                    "mvalue":     m.Value,
                    "confidence": m.Confidence,
                    "updated_at": now,
                }).
                FirstOrCreate(memory)

            if result.Error != nil {
                return result.Error
            }
        }

        return nil
    })
}
```

**优势**：
- 自动事务管理
- FirstOrCreate 简化 UPSERT 逻辑
- 更好的错误处理

### 6. 新增功能

#### SetDebug 方法

```go
func (s *Store) SetDebug(debug bool) {
    if debug {
        s.db.Logger = logger.Default.LogMode(logger.Info)
    } else {
        s.db.Logger = logger.Default.LogMode(logger.Silent)
    }
}
```

**用途**：动态控制 SQL 日志输出

#### 新增便利方法

```go
// GetMemoriesByType 根据类型获取记忆
func (s *Store) GetMemoriesByType(ctx context.Context, userID, mtype string) ([]Memory, error)

// DeleteMemory 删除记忆（软删除）
func (s *Store) DeleteMemory(ctx context.Context, userID string, mtype, mkey, owner string) error

// GetMemoryCount 获取记忆总数
func (s *Store) GetMemoryCount(ctx context.Context, userID string) (int64, error)

// ExportMemories 导出所有记忆为 JSON
func (s *Store) ExportMemories(ctx context.Context, userID string) (string, error)
```

## GORM 优势

### 1. 类型安全
- 编译时检查字段类型
- 避免 SQL 注入
- IDE 自动补全

### 2. 自动迁移
- 自动创建表和索引
- 自动添加新字段
- 向后兼容

### 3. 关联查询
- 支持 Preload、Joins
- 自动处理关联关系
- 减少 N+1 查询

### 4. 钩子系统
- BeforeCreate、AfterCreate
- BeforeUpdate、AfterUpdate
- BeforeDelete、AfterDelete

### 5. 事务支持
- 自动事务管理
- 嵌套事务支持
- SavePoint 支持

### 6. 软删除
- 自动处理 DeletedAt
- 查询自动过滤已删除记录
- 可恢复删除的记录

### 7. 链式调用
- 更易读的查询构建
- 动态条件组合
- 方法复用

## 性能考虑

### 1. 连接池
```go
sqlDB, err := s.db.DB()
sqlDB.SetMaxIdleConns(10)
sqlDB.SetMaxOpenConns(100)
sqlDB.SetConnMaxLifetime(time.Hour)
```

### 2. 批量操作
```go
// 批量插入
s.db.CreateInBatches(memories, 100)

// 批量更新
s.db.Model(&Memory{}).Where("user_id = ?", userID).Updates(map[string]interface{}{
    "access_count": gorm.Expr("access_count + 1"),
})
```

### 3. 预加载
```go
// 避免 N+1 查询
s.db.Preload("Associations").Find(&memories)
```

## 向后兼容性

### 1. 表结构兼容
- GORM 使用相同的表名和字段名
- 自动迁移不会破坏现有数据
- 支持从旧数据库无缝升级

### 2. API 兼容
- 所有公开方法签名保持不变
- 返回类型保持一致
- 行为保持一致

### 3. 数据兼容
- 现有数据无需迁移
- 自动适配现有表结构
- 支持增量升级

## 测试验证

### 编译测试
```bash
✓ go build ./cmd/chat
✓ go build ./cmd/api
✓ go build ./cmd/ingest
```

### 功能测试
建议测试以下功能：
1. 保存和读取聊天消息
2. 插入和更新记忆
3. 查询记忆统计
4. 软删除功能
5. 事务回滚

## 最佳实践

### 1. 使用 Context
```go
s.db.WithContext(ctx).Find(&memories)
```

### 2. 错误处理
```go
if err := s.db.Create(&memory).Error; err != nil {
    return fmt.Errorf("failed to create memory: %w", err)
}
```

### 3. 事务使用
```go
s.db.Transaction(func(tx *gorm.DB) error {
    // 操作
    return nil
})
```

### 4. 避免 N+1
```go
s.db.Preload("Associations").Find(&records)
```

### 5. 索引优化
```go
type Memory struct {
    UserID string `gorm:"index:idx_user_type,priority:1"`
    Type   string `gorm:"index:idx_user_type,priority:2"`
}
```

## 未来扩展

### 1. 多数据库支持
- PostgreSQL
- MySQL
- SQL Server

### 2. 读写分离
```go
s.db.Clauses(dbresolver.Write).Create(&memory)
s.db.Clauses(dbresolver.Read).Find(&memories)
```

### 3. 分片支持
```go
s.db.Use(sharding.Register(sharding.Config{
    ShardingKey: "user_id",
}))
```

### 4. 缓存集成
```go
s.db.Use(cacheplugin.New(&cacheplugin.Config{
    Cache: cache,
}))
```

## 相关文档

- [GORM 官方文档](https://gorm.io/docs/)
- [GORM SQLite 驱动](https://github.com/go-gorm/sqlite)
- [GORM 最佳实践](https://gorm.io/docs/performance.html)

## 技术支持

如有问题，请查看：
- `internal/memory/types.go` - 模型定义
- `internal/memory/sqlite.go` - Store 实现
- GitHub Issues - 提交问题和建议
