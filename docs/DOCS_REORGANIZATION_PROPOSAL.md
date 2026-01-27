# 文档组织方案

## 📊 当前状态

根目录有 **26个 Markdown 文件**，过于混乱，影响项目可读性。

## 🎯 建议的文档结构

```
AIAgent/
├── README.md                    # 保留：项目主文档
├── CLAUDE.md                    # 保留：Claude Code 指令（被系统引用）
├── docs/                        # 新建：文档目录
│   ├── getting-started/         # 快速开始
│   │   ├── quickstart.md
│   │   └── configuration.md
│   ├── features/                # 功能文档
│   │   ├── deepseek.md
│   │   ├── memory-system.md
│   │   └── tools.md
│   ├── architecture/            # 架构文档
│   │   ├── overview.md
│   │   ├── modularization.md
│   │   └── optimization-plan.md
│   ├── development/             # 开发文档
│   │   ├── implementation-roadmap.md
│   │   └── quick-wins/
│   │       ├── README.md
│   │       ├── 01-calculator.md
│   │       ├── 02-streaming.md
│   │       ├── 03-memory-stats.md
│   │       └── 04-context-window.md
│   └── archive/                 # 历史文档
│       ├── memory-analysis.md
│       ├── memory-optimization.md
│       └── config-updates.md
```

## 📁 文件分类与迁移

### 保留在根目录 (2个)
- ✅ `README.md` - 项目主文档
- ✅ `CLAUDE.md` - Claude Code 指令（系统引用）

### 迁移到 docs/getting-started/ (3个)
- `QUICKSTART.md` → `docs/getting-started/quickstart.md`
- `CONFIG.md` → `docs/getting-started/configuration.md`
- `CONFIG_UPDATE.md` → `docs/archive/config-updates.md`

### 迁移到 docs/features/ (2个)
- `DEEPSEEK.md` → `docs/features/deepseek.md`
- `FEATURE_UPDATE.md` → `docs/features/feature-updates.md`

### 迁移到 docs/architecture/ (4个)
- `MODULARIZATION_REPORT.md` → `docs/architecture/modularization.md`
- `OPTIMIZATION_PLAN.md` → `docs/architecture/optimization-plan.md`
- `OPTIMIZATION_SUMMARY.md` → `docs/architecture/optimization-summary.md`
- `SIMPLIFICATION.md` → `docs/architecture/simplification.md`

### 迁移到 docs/development/ (2个)
- `IMPLEMENTATION.md` → `docs/development/implementation.md`
- `IMPLEMENTATION_ROADMAP.md` → `docs/development/roadmap.md`

### 迁移到 docs/development/quick-wins/ (9个)
- `QUICK_WIN_EXAMPLES.md` → `docs/development/quick-wins/README.md`
- `QUICK_WIN_1_COMPLETED.md` + `QUICK_WIN_1_SUCCESS.md` → `docs/development/quick-wins/01-calculator.md`
- `QUICK_WIN_2_COMPLETED.md` + `QUICK_WIN_2_SUCCESS.md` → `docs/development/quick-wins/02-streaming.md`
- `QUICK_WIN_3_COMPLETED.md` + `QUICK_WIN_3_SUCCESS.md` → `docs/development/quick-wins/03-memory-stats.md`
- `QUICK_WIN_4_COMPLETED.md` + `QUICK_WIN_4_SUCCESS.md` → `docs/development/quick-wins/04-context-window.md`

### 迁移到 docs/archive/ (4个)
- `MEMORY_ANALYSIS.md` → `docs/archive/memory-analysis.md`
- `MEMORY_CLASSIFICATION.md` → `docs/archive/memory-classification.md`
- `MEMORY_FIX_REPORT.md` → `docs/archive/memory-fix-report.md`
- `MEMORY_OPTIMIZATION_P1.md` → `docs/archive/memory-optimization-p1.md`

## 📝 迁移后的根目录

```
AIAgent/
├── README.md                    # 项目概述
├── CLAUDE.md                    # Claude Code 指令
├── docs/                        # 所有文档
├── cmd/                         # 命令行工具
├── internal/                    # 内部包
├── config.yaml                  # 配置文件
├── go.mod                       # Go 模块
└── ...
```

## ✨ 优势

### 1. 清晰的组织结构
- 根目录只保留必要文件
- 文档按类型分类
- 易于查找和维护

### 2. 更好的可读性
- 新用户不会被大量文档淹没
- 文档层次清晰
- 历史文档归档

### 3. 便于维护
- 相关文档集中管理
- 减少根目录混乱
- 符合开源项目最佳实践

### 4. 合并重复内容
- Quick Win 的 COMPLETED 和 SUCCESS 文档可以合并
- 减少文档数量
- 避免信息重复

## 🚀 实施步骤

### 步骤1: 创建目录结构
```bash
mkdir -p docs/{getting-started,features,architecture,development/quick-wins,archive}
```

### 步骤2: 迁移文件
```bash
# Getting Started
mv QUICKSTART.md docs/getting-started/quickstart.md
mv CONFIG.md docs/getting-started/configuration.md

# Features
mv DEEPSEEK.md docs/features/deepseek.md
mv FEATURE_UPDATE.md docs/features/feature-updates.md

# Architecture
mv MODULARIZATION_REPORT.md docs/architecture/modularization.md
mv OPTIMIZATION_PLAN.md docs/architecture/optimization-plan.md
mv OPTIMIZATION_SUMMARY.md docs/architecture/optimization-summary.md
mv SIMPLIFICATION.md docs/architecture/simplification.md

# Development
mv IMPLEMENTATION.md docs/development/implementation.md
mv IMPLEMENTATION_ROADMAP.md docs/development/roadmap.md

# Quick Wins (需要合并)
# ... (见下方详细步骤)

# Archive
mv MEMORY_*.md docs/archive/
mv CONFIG_UPDATE.md docs/archive/config-updates.md
```

### 步骤3: 更新 README.md
添加文档导航：
```markdown
## 📚 文档

- [快速开始](docs/getting-started/quickstart.md)
- [配置指南](docs/getting-started/configuration.md)
- [架构设计](docs/architecture/)
- [开发指南](docs/development/)
```

### 步骤4: 更新 CLAUDE.md
更新文档路径引用（如果有）

## ⚠️ 注意事项

1. **CLAUDE.md 必须保留在根目录**
   - 被 Claude Code 系统引用
   - 路径硬编码在系统中

2. **README.md 保留在根目录**
   - GitHub 默认显示
   - 项目入口文档

3. **更新内部链接**
   - 文档间的相对链接需要更新
   - 确保所有引用正确

4. **Git 历史**
   - 使用 `git mv` 保留文件历史
   - 避免使用 `mv` 命令

## 📊 效果对比

| 指标 | 重组前 | 重组后 | 改善 |
|------|--------|--------|------|
| 根目录文件数 | 26个 | 2个 | **-92%** |
| 文档组织 | 混乱 | 清晰 | ✅ |
| 查找效率 | 低 | 高 | ✅ |
| 维护性 | 差 | 好 | ✅ |

## 🎯 建议

**立即执行**：
1. 创建 docs 目录结构
2. 迁移文档文件
3. 合并重复的 Quick Win 文档
4. 更新 README.md 添加文档导航

**收益**：
- 根目录从26个文件减少到2个
- 文档结构清晰，易于维护
- 符合开源项目最佳实践
- 提升项目专业度

---

**是否立即执行文档重组？**
