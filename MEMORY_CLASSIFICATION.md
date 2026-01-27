# 记忆分类标准与优化建议

## 一、当前实现分析

### 1.1 短期记忆（Short-term Memory）

**当前实现**：
```go
type WindowMemory struct {
    N     int      // 窗口大小（默认 8 轮）
    Turns []Turn   // 完整对话轮次
}
```

**存储内容**：
- 最近 N 轮的完整对话
- 包含用户输入和助手回复
- **无过滤、无筛选**

**使用场景**：
- 维持对话连贯性
- 理解上下文引用（"它"、"那个"等）
- 保持话题连续性

---

### 1.2 长期记忆（Long-term Memory）

**当前实现**：
- **SQLite**：结构化记忆（key-value pairs）
- **Qdrant**：语义向量记忆

**存储内容**：
```
提取规则：
- 只提取"稳定、可复用的信息"
- 不保存"短期一次性内容"
- 类型：identity/preference/goal/tool/constraint/fact/activity/duration
```

**问题**：
❌ "稳定、可复用" vs "短期一次性" 边界模糊
❌ 没有明确的时间维度
❌ 某些类型定义不清（activity, duration）
❌ 缺少重要性评估标准

---

## 二、业界标准与最佳实践

### 2.1 认知科学视角

人类记忆系统分为三层：

```
感觉记忆（Sensory Memory）
    ↓ 注意力筛选
工作记忆（Working Memory）
    ↓ 编码与巩固
长期记忆（Long-term Memory）
```

**对应到 AI Agent**：

| 人类记忆 | AI Agent | 保留时间 | 容量 | 特点 |
|---------|----------|---------|------|------|
| 感觉记忆 | 当前输入 | 瞬时 | 无限 | 原始输入 |
| 工作记忆 | 短期记忆 | 会话期间 | 有限（8-10轮） | 维持上下文 |
| 长期记忆 | 长期记忆 | 永久 | 无限 | 知识积累 |

---

### 2.2 LangChain 的记忆分类

```python
# 1. ConversationBufferMemory（对话缓冲记忆）
# 类似你的 WindowMemory
memory = ConversationBufferMemory()

# 2. ConversationSummaryMemory（对话摘要记忆）
# 压缩旧对话为摘要
memory = ConversationSummaryMemory(llm=llm)

# 3. ConversationKGMemory（知识图谱记忆）
# 提取实体和关系
memory = ConversationKGMemory(llm=llm)

# 4. VectorStoreMemory（向量存储记忆）
# 类似你的 Qdrant
memory = VectorStoreMemory(vectorstore=vectorstore)
```

---

### 2.3 MemGPT 的分层记忆

```
┌─────────────────────────────────────┐
│ 核心记忆（Core Memory）             │
│ - 永久、关键信息                     │
│ - 身份、目标、约束                   │
│ - 类似你的 identity/goal/constraint  │
└─────────────────────────────────────┘
           ↓
┌─────────────────────────────────────┐
│ 召回记忆（Recall Memory）           │
│ - 可检索的历史对话                   │
│ - 类似你的 Qdrant                    │
└─────────────────────────────────────┘
           ↓
┌─────────────────────────────────────┐
│ 归档记忆（Archival Memory）         │
│ - 长期存储、低频访问                 │
│ - 可以压缩或摘要                     │
└─────────────────────────────────────┘
```

---

## 三、明确的分类标准

### 3.1 短期记忆标准

**定义**：维持当前对话连贯性所需的临时信息

**判断标准**：

| 维度 | 标准 | 示例 |
|------|------|------|
| **时间性** | 仅在当前会话有效 | "刚才说的那个"、"上面提到的" |
| **引用性** | 包含代词或指代 | "它"、"那个"、"这个问题" |
| **临时性** | 一次性任务或状态 | "帮我计算一下"、"等一下" |
| **上下文依赖** | 脱离上下文无意义 | "继续"、"再说一遍" |

**存储内容**：
```
✅ 完整对话轮次（用户 + 助手）
✅ 原始文本（不提取、不压缩）
✅ 时间戳
✅ 轮次编号
```

**不存储**：
```
❌ 不提取结构化信息
❌ 不做语义分析
❌ 不做重要性评估
```

**生命周期**：
- 会话结束后清空
- 或达到窗口大小后滚动删除

---

### 3.2 长期记忆标准

**定义**：跨会话可复用的稳定知识

**判断标准（STABLE 原则）**：

#### S - Stable（稳定性）
- ✅ 不会频繁变化的信息
- ✅ 示例：姓名、职业、偏好
- ❌ 反例：当前心情、临时想法

#### T - Timeless（时间无关）
- ✅ 不依赖特定时间点
- ✅ 示例：喜欢蓝色、是程序员
- ❌ 反例：今天很累、刚吃了饭

#### A - Actionable（可操作）
- ✅ 未来可以用于个性化
- ✅ 示例：编程语言偏好、工作习惯
- ❌ 反例：随口一说的话

#### B - Broad（广泛适用）
- ✅ 在多个场景下有用
- ✅ 示例：技能、兴趣、价值观
- ❌ 反例：某次具体对话的细节

#### L - Long-lasting（持久性）
- ✅ 预期长期有效
- ✅ 示例：教育背景、家庭状况
- ❌ 反例：临时项目、短期计划

#### E - Explicit（明确性）
- ✅ 用户明确表达的
- ✅ 示例："我是..."、"我喜欢..."
- ❌ 反例：助手的推测或假设

---

### 3.3 记忆类型详细定义

#### 1. Identity（身份信息）⭐⭐⭐⭐⭐
**定义**：用户或 agent 的基本身份属性

**应该存储**：
```
✅ 姓名：name = "张三"
✅ 职业：occupation = "软件工程师"
✅ 年龄：age = "30"
✅ 性别：gender = "男"
✅ 地区：location = "北京"
✅ 教育：education = "计算机硕士"
✅ 技能：skill = "Python"
```

**不应该存储**：
```
❌ 临时状态："今天很累"
❌ 一次性信息："刚到公司"
❌ 推测信息："可能是程序员"（除非用户确认）
```

**置信度要求**：≥ 0.8

---

#### 2. Preference（偏好）⭐⭐⭐⭐
**定义**：用户的喜好、习惯、风格

**应该存储**：
```
✅ 颜色偏好：color = "蓝色"
✅ 编程语言：language = "Python"
✅ 工作时间：work_time = "早上"
✅ 沟通风格：style = "简洁直接"
✅ 工具偏好：editor = "VSCode"
```

**不应该存储**：
```
❌ 临时选择："今天想吃面"
❌ 情境依赖："现在想休息"
❌ 不确定的："可能喜欢"
```

**置信度要求**：≥ 0.7

---

#### 3. Goal（目标）⭐⭐⭐⭐
**定义**：用户的长期目标、计划

**应该存储**：
```
✅ 学习目标：learn = "机器学习"
✅ 职业目标：career = "成为架构师"
✅ 项目目标：project = "开发 AI 助手"
✅ 个人目标：personal = "提升英语"
```

**不应该存储**：
```
❌ 短期任务："今天要写代码"
❌ 临时想法："想学一下 Go"
❌ 已完成的："学会了 Python"（应该转为 skill）
```

**置信度要求**：≥ 0.7

---

#### 4. Tool（工具/环境）⭐⭐⭐
**定义**：用户常用的工具、平台、环境

**应该存储**：
```
✅ 操作系统：os = "macOS"
✅ 编辑器：editor = "VSCode"
✅ 框架：framework = "Django"
✅ 数据库：database = "PostgreSQL"
✅ 云平台：cloud = "AWS"
```

**不应该存储**：
```
❌ 临时工具："试用了 Vim"
❌ 一次性使用："用过一次 Sublime"
```

**置信度要求**：≥ 0.7

---

#### 5. Constraint（约束）⭐⭐⭐
**定义**：用户的限制、禁忌、规则

**应该存储**：
```
✅ 时间约束：available = "工作日晚上"
✅ 技术约束：no_use = "不用 Java"
✅ 隐私约束：privacy = "不分享个人信息"
✅ 预算约束：budget = "开源优先"
```

**不应该存储**：
```
❌ 临时限制："今天没时间"
❌ 情境约束："现在不方便"
```

**置信度要求**：≥ 0.8

---

#### 6. Fact（事实）⭐⭐
**定义**：客观事实、知识点

**应该存储**：
```
✅ 工作事实：company = "某科技公司"
✅ 项目事实：project_name = "AI Agent"
✅ 技术事实：uses_tech = "Go + Python"
```

**不应该存储**：
```
❌ 临时事实："今天下雨"
❌ 一次性事件："昨天开会"
❌ 常识性事实："Python 是编程语言"
```

**置信度要求**：≥ 0.6

---

#### 7. Activity（活动）⭐
**定义**：用户的重复性活动、习惯

**应该存储**：
```
✅ 定期活动：routine = "每天晨跑"
✅ 工作习惯：habit = "早上写代码"
✅ 学习习惯：study = "晚上看书"
```

**不应该存储**：
```
❌ 一次性活动："今天去了健身房"
❌ 临时事件："刚开了个会"
```

**置信度要求**：≥ 0.6

**⚠️ 问题**：这个类型容易与 preference 混淆，建议合并到 preference

---

#### 8. Duration（时长）⭐
**定义**：时间相关的信息

**应该存储**：
```
✅ 工作时长：work_hours = "8小时"
✅ 经验时长：experience = "5年"
✅ 学习时长：study_time = "每天2小时"
```

**不应该存储**：
```
❌ 临时时长："今天工作了10小时"
❌ 一次性时长："这次花了3小时"
```

**置信度要求**：≥ 0.6

**⚠️ 问题**：这个类型定义模糊，建议合并到其他类型

---

## 四、优化建议

### 4.1 简化记忆类型

**当前 8 种类型过多，建议简化为 5 种**：

```
1. identity    - 身份信息（保留）⭐⭐⭐⭐⭐
2. preference  - 偏好习惯（保留，合并 activity）⭐⭐⭐⭐
3. goal        - 目标计划（保留）⭐⭐⭐⭐
4. context     - 上下文信息（新增，合并 tool/constraint/fact/duration）⭐⭐⭐
5. knowledge   - 知识技能（新增）⭐⭐⭐
```

**映射关系**：
```
identity    → identity（不变）
preference  → preference + activity
goal        → goal（不变）
tool        → context
constraint  → context
fact        → context
duration    → context（或删除）
新增 knowledge → 技能、知识、经验
```

---

### 4.2 添加时间维度

**建议添加记忆的时间属性**：

```go
type ExtractedMemory struct {
    Type       string
    Key        string
    Value      string
    Confidence float64
    Owner      string

    // 新增字段
    Temporal   string    // "permanent" | "long-term" | "medium-term"
    ExpiresAt  *time.Time // 过期时间（可选）
    CreatedAt  time.Time  // 创建时间
    AccessedAt time.Time  // 最后访问时间
}
```

**时间分类**：

| 类型 | 定义 | 保留时间 | 示例 |
|------|------|---------|------|
| permanent | 永久性信息 | 永久 | 姓名、出生日期 |
| long-term | 长期信息 | 1年+ | 职业、技能、偏好 |
| medium-term | 中期信息 | 1-6个月 | 当前项目、短期目标 |

---

### 4.3 改进提取 Prompt

**当前 prompt 问题**：
- 规则过多（200+ 行）
- 定义模糊
- 难以维护

**建议的新 prompt**：

```
你是记忆提取器。从对话中提取值得长期保存的信息。

【提取标准 - STABLE 原则】
只提取满足以下所有条件的信息：
1. Stable（稳定）：不会频繁变化
2. Timeless（时间无关）：不依赖特定时间点
3. Actionable（可操作）：未来可用于个性化
4. Broad（广泛适用）：在多个场景有用
5. Long-lasting（持久）：预期长期有效
6. Explicit（明确）：用户明确表达的

【记忆类型】
1. identity - 身份信息（姓名、职业、年龄等）
2. preference - 偏好习惯（喜好、风格、习惯）
3. goal - 目标计划（学习目标、职业目标）
4. context - 上下文信息（工具、环境、约束）
5. knowledge - 知识技能（技能、经验、专长）

【不要提取】
❌ 临时状态："今天很累"
❌ 一次性事件："刚吃了饭"
❌ 推测信息："可能喜欢"
❌ 敏感信息：密码、身份证等

【输出格式】
{
  "memories": [
    {
      "type": "identity",
      "key": "name",
      "value": "张三",
      "confidence": 1.0,
      "temporal": "permanent",
      "owner": "user"
    }
  ]
}
```

---

### 4.4 添加记忆重要性评分

**建议添加重要性评分系统**：

```go
type MemoryImportance int

const (
    Critical   MemoryImportance = 5  // 关键信息（identity）
    High       MemoryImportance = 4  // 高重要性（preference, goal）
    Medium     MemoryImportance = 3  // 中等重要性（context）
    Low        MemoryImportance = 2  // 低重要性（knowledge）
    Trivial    MemoryImportance = 1  // 琐碎信息（不应存储）
)

func calculateImportance(m ExtractedMemory) MemoryImportance {
    switch m.Type {
    case "identity":
        return Critical
    case "preference", "goal":
        return High
    case "context":
        return Medium
    case "knowledge":
        return Low
    default:
        return Trivial
    }
}
```

---

### 4.5 实现记忆生命周期管理

```go
// 记忆状态
type MemoryState string

const (
    Active    MemoryState = "active"     // 活跃（经常访问）
    Dormant   MemoryState = "dormant"    // 休眠（很少访问）
    Archived  MemoryState = "archived"   // 归档（长期不用）
    Expired   MemoryState = "expired"    // 过期（应删除）
)

// 记忆生命周期管理
func manageMemoryLifecycle(db *sql.DB) {
    // 1. 标记休眠记忆（30天未访问）
    db.Exec(`
        UPDATE memories
        SET state = 'dormant'
        WHERE accessed_at < datetime('now', '-30 days')
          AND state = 'active'
    `)

    // 2. 归档休眠记忆（90天未访问）
    db.Exec(`
        UPDATE memories
        SET state = 'archived'
        WHERE accessed_at < datetime('now', '-90 days')
          AND state = 'dormant'
    `)

    // 3. 删除过期记忆（1年未访问且置信度低）
    db.Exec(`
        DELETE FROM memories
        WHERE accessed_at < datetime('now', '-365 days')
          AND confidence < 0.5
          AND state = 'archived'
    `)
}
```

---

## 五、实施建议

### 优先级 1（立即实施）

1. **明确提取标准**
   - 更新 prompt，使用 STABLE 原则
   - 添加清晰的示例

2. **简化记忆类型**
   - 从 8 种减少到 5 种
   - 更新数据库 schema

### 优先级 2（短期实施）

3. **添加时间维度**
   - 添加 temporal 字段
   - 实现过期机制

4. **添加重要性评分**
   - 实现评分算法
   - 优化召回策略

### 优先级 3（长期实施）

5. **生命周期管理**
   - 实现状态转换
   - 定期清理

6. **记忆质量监控**
   - 统计记忆使用率
   - 识别低质量记忆

---

## 六、总结

### 当前问题
❌ 短期/长期记忆边界模糊
❌ 记忆类型定义不清
❌ 缺少时间维度
❌ 缺少重要性评估

### 改进后
✅ 清晰的 STABLE 原则
✅ 简化的 5 种类型
✅ 完整的时间维度
✅ 重要性评分系统
✅ 生命周期管理

### 核心原则

**短期记忆**：
- 维持对话连贯性
- 会话期间有效
- 不做提取和分析

**长期记忆**：
- 跨会话可复用
- 满足 STABLE 原则
- 有明确的类型和重要性
