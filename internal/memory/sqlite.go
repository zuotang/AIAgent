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
  UNIQUE(user_id, mtype, mkey, owner)
);

`
	_, err := s.db.Exec(ddl)
	return err
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
	for rows.Next() {
		var t, k, v, owner string
		var c float64
		if err := rows.Scan(&t, &k, &v, &c, &owner); err != nil {
			return "", err
		}
		out += "- " + owner + ": " + t + "." + k + " = " + v + " (conf=" + format2(c) + ")\n"
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
