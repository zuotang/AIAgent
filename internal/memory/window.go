package memory

import (
	"fmt"
	"strings"

	"agent-langchain/internal/utils"
)

// Turn 对话轮次
type Turn struct {
	User      string
	Assistant string
}

// WindowMemory 滑动窗口记忆
type WindowMemory struct {
	N     int
	Turns []Turn
}

// NewWindowMemory 创建新的窗口记忆
func NewWindowMemory(n int) *WindowMemory {
	return &WindowMemory{N: n}
}

// Add 添加一轮对话
func (m *WindowMemory) Add(user, assistant string) {
	user = utils.PreprocessLite(user)
	assistant = utils.PreprocessLite(assistant)
	m.Turns = append(m.Turns, Turn{User: user, Assistant: assistant})
	if len(m.Turns) > m.N {
		m.Turns = m.Turns[len(m.Turns)-m.N:]
	}
}

// String 格式化为字符串
func (m *WindowMemory) String() string {
	var b strings.Builder
	for i, t := range m.Turns {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("User: %s\n", t.User))
		b.WriteString(fmt.Sprintf("Assistant: %s\n", t.Assistant))
	}
	return b.String()
}

// Clear 清空窗口
func (m *WindowMemory) Clear() {
	m.Turns = nil
}

// Size 返回当前窗口大小
func (m *WindowMemory) Size() int {
	return len(m.Turns)
}
