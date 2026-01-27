package memory

import (
	"context"
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) init() error {
	ddl := `
CREATE TABLE IF NOT EXISTS memories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id TEXT NOT NULL,
  mtype TEXT NOT NULL,
  mkey  TEXT NOT NULL,
  mvalue TEXT NOT NULL,
  confidence REAL NOT NULL DEFAULT 0.7,
  owner TEXT NOT NULL DEFAULT 'user',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  access_count INTEGER NOT NULL DEFAULT 0,
  last_accessed_at DATETIME,
  UNIQUE(user_id, mtype, mkey, owner)
);

`
	_, err := s.db.Exec(ddl)
	if err != nil {
		return err
	}

	// 添加新字段（如果表已存在但没有这些字段）
	// SQLite的ALTER TABLE ADD COLUMN是幂等的，如果列已存在会报错但不影响
	s.db.Exec(`ALTER TABLE memories ADD COLUMN access_count INTEGER NOT NULL DEFAULT 0`)
	s.db.Exec(`ALTER TABLE memories ADD COLUMN last_accessed_at DATETIME`)

	return nil
}

// UpsertExtractedMemories：把“记忆提取器”输出写入 SQLite（可覆盖更新）
func (s *Store) UpsertExtractedMemories(ctx context.Context, userID string, ms []ExtractedMemory) error {
	for _, m := range ms {
		if m.Confidence < 0.65 {
			continue
		}
		if m.Type == "" || m.Key == "" || m.Value == "" {
			continue
		}
		// 确保owner字段有值
		owner := m.Owner
		if owner == "" {
			owner = "user"
		}
		_, err := s.db.ExecContext(ctx, `
INSERT INTO memories(user_id,mtype,mkey,mvalue,confidence,owner,updated_at)
VALUES(?,?,?,?,?,?,?)
ON CONFLICT(user_id,mtype,mkey,owner) DO UPDATE SET
 mvalue=excluded.mvalue,
 confidence=excluded.confidence,
 owner=excluded.owner,
 updated_at=excluded.updated_at
`, userID, m.Type, m.Key, m.Value, m.Confidence, owner, time.Now().Format(time.RFC3339))
		if err != nil {
			return err
		}
	}
	return nil
}

// RenderStructuredMemory：把 SQLite 里的结构化记忆渲染成一段文本，塞给 system/prompt 用
func (s *Store) RenderStructuredMemory(ctx context.Context, userID string, limit int) (string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT mtype,mkey,mvalue,confidence,owner
FROM memories
WHERE user_id=?
ORDER BY
  CASE mtype
    WHEN 'identity' THEN 1
    WHEN 'preference' THEN 2
    WHEN 'goal' THEN 3
    WHEN 'knowledge' THEN 4
    WHEN 'context' THEN 5
    ELSE 9
  END,
  confidence DESC,
  updated_at DESC
LIMIT ?`, userID, limit)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	out := "【结构化长期记忆(SQLite)】\n"
	var accessedMemories []struct {
		mtype string
		mkey  string
		owner string
	}

	for rows.Next() {
		var t, k, v, owner string
		var c float64
		if err := rows.Scan(&t, &k, &v, &c, &owner); err != nil {
			return "", err
		}
		out += "- " + owner + ": " + t + "." + k + " = " + v + " (conf=" + format2(c) + ")\n"

		// 记录访问的记忆
		accessedMemories = append(accessedMemories, struct {
			mtype string
			mkey  string
			owner string
		}{t, k, owner})
	}

	// 异步更新访问统计
	if len(accessedMemories) > 0 {
		go s.updateAccessStats(context.Background(), userID, accessedMemories)
	}

	return out, nil
}

func format2(f float64) string {
	// 小工具：避免 fmt 依赖；你也可以直接用 fmt.Sprintf("%.2f", f)
	// 这里简单粗暴：
	if f >= 1 {
		return "1.00"
	}
	if f <= 0 {
		return "0.00"
	}
	// 保留两位
	n := int(f*100 + 0.5)
	return string('0'+(n/100)) + "." + twoDigits(n%100)
}
func twoDigits(n int) string {
	return string('0'+(n/10)) + string('0'+(n%10))
}

// updateAccessStats 异步更新访问统计
func (s *Store) updateAccessStats(ctx context.Context, userID string, memories []struct {
	mtype string
	mkey  string
	owner string
}) {
	if len(memories) == 0 {
		return
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE memories
		SET access_count = access_count + 1,
		    last_accessed_at = ?
		WHERE user_id = ? AND mtype = ? AND mkey = ? AND owner = ?
	`)
	if err != nil {
		return
	}
	defer stmt.Close()

	now := time.Now().Format(time.RFC3339)
	for _, m := range memories {
		stmt.ExecContext(ctx, now, userID, m.mtype, m.mkey, m.owner)
	}

	tx.Commit()
}

// GetTopAccessedMemories 获取最常访问的记忆
func (s *Store) GetTopAccessedMemories(ctx context.Context, userID string, limit int) ([]MemoryStats, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT mtype, mkey, mvalue, owner, confidence, access_count, last_accessed_at, updated_at
		FROM memories
		WHERE user_id = ? AND access_count > 0
		ORDER BY access_count DESC, last_accessed_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []MemoryStats
	for rows.Next() {
		var s MemoryStats
		var lastAccessed sql.NullString
		if err := rows.Scan(&s.Type, &s.Key, &s.Value, &s.Owner, &s.Confidence, &s.AccessCount, &lastAccessed, &s.UpdatedAt); err != nil {
			continue
		}
		if lastAccessed.Valid {
			s.LastAccessed = lastAccessed.String
		}
		stats = append(stats, s)
	}

	return stats, nil
}

// MemoryStats 记忆统计信息
type MemoryStats struct {
	Type         string
	Key          string
	Value        string
	Owner        string
	Confidence   float64
	AccessCount  int
	LastAccessed string
	UpdatedAt    string
}
