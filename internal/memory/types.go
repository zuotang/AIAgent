package memory

// Profile 是给 system 注入用的长期设定（你之前的 agent/user 结构）
type Profile struct {
	Agent map[string]any `json:"agent"`
	User  map[string]any `json:"user"`
}

type ExtractedMemory struct {
	Type       string  `json:"type"`        // profile | preference | rule | fact
	Key        string  `json:"key"`         // e.g. "language"
	Value      string  `json:"value"`       // e.g. "Chinese"
	Confidence float64 `json:"confidence"`  // 0~1
	AlsoVector bool    `json:"also_vector"` // 是否也写入 Qdrant（便于检索）
	Text       string  `json:"text"`        // 写入 Qdrant 的语义文本（可选）
	Owner      string  `json:"owner"`       // user | agent，标识记忆的所有者
}
