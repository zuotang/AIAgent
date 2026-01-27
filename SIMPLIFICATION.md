# 配置简化说明

## 变更内容

### 移除的功能

已移除所有命令行参数（除了 `-config`），所有配置必须通过配置文件管理。

### 之前（旧版本）

```bash
# 可以通过命令行参数覆盖配置
./chat.exe -provider deepseek -deepseek-key sk-xxxxx -debug -timeout 120
```

### 现在（新版本）

```bash
# 只能指定配置文件
./chat.exe -config config.yaml
```

所有配置必须在 `config.yaml` 中设置：

```yaml
provider: deepseek

deepseek:
  base_url: https://api.deepseek.com/v1
  api_key: "sk-xxxxx"
  chat_model: deepseek-chat

debug: true
timeout: 120
```

## 优势

### 1. 配置更加规范
- 所有配置集中在一个地方
- 避免配置来源混乱（命令行 vs 配置文件）
- 更容易审查和管理配置

### 2. 代码更简洁
- 减少了大量的命令行参数处理代码
- 减少了参数覆盖逻辑
- 更容易维护

### 3. 更适合生产环境
- 配置文件可以版本控制（去除敏感信息后）
- 更容易在不同环境间切换
- 减少人为错误（不会忘记某个参数）

### 4. 更好的安全性
- API 密钥等敏感信息不会出现在命令行历史中
- 配置文件权限更容易控制

## 使用方法

### 基本使用

```bash
# 使用默认配置文件 config.yaml
./chat.exe

# 使用指定配置文件
./chat.exe -config production.yaml
```

### 多环境配置

创建不同的配置文件：

```bash
# 开发环境
config.dev.yaml

# 测试环境
config.test.yaml

# 生产环境
config.prod.yaml
```

运行时指定：

```bash
# 开发
./chat.exe -config config.dev.yaml

# 生产
./chat.exe -config config.prod.yaml
```

### 调试模式

在配置文件中设置：

```yaml
debug: true
```

### 切换 LLM 提供商

编辑配置文件：

```yaml
# 使用 Ollama
provider: ollama

# 或使用 DeepSeek
provider: deepseek
```

## 迁移指南

如果你之前使用命令行参数，需要将它们移到配置文件中：

### 示例 1: 调试模式

**之前**:
```bash
./chat.exe -debug
```

**现在**:
```yaml
# config.yaml
debug: true
```
```bash
./chat.exe
```

### 示例 2: 使用 DeepSeek

**之前**:
```bash
./chat.exe -provider deepseek -deepseek-key sk-xxxxx
```

**现在**:
```yaml
# config.yaml
provider: deepseek

deepseek:
  base_url: https://api.deepseek.com/v1
  api_key: "sk-xxxxx"
  chat_model: deepseek-chat
```
```bash
./chat.exe
```

### 示例 3: 自定义超时

**之前**:
```bash
./chat.exe -timeout 120
```

**现在**:
```yaml
# config.yaml
timeout: 120
```
```bash
./chat.exe
```

## 配置文件模板

### 完整配置示例

```yaml
# LLM 提供商
provider: deepseek

# Ollama 配置（用于 embedding）
ollama:
  base_url: http://127.0.0.1:11434
  chat_model: gemma3:12b
  embed_model: nomic-embed-text

# DeepSeek 配置
deepseek:
  base_url: https://api.deepseek.com/v1
  api_key: "sk-your-api-key"
  chat_model: deepseek-chat

# Qdrant 配置
qdrant:
  base_url: http://127.0.0.1:6333
  api_key: ""
  collection: memories
  top_k: 6

# 数据库配置
database:
  path: memory.db

# 记忆配置
memory:
  window_size: 8
  extractor_model: ""

# 调试和超时
debug: false
timeout: 60
```

## 常见问题

### Q: 如何快速切换配置？

创建多个配置文件，使用 `-config` 参数切换：

```bash
./chat.exe -config config.dev.yaml
./chat.exe -config config.prod.yaml
```

### Q: 如何临时修改配置？

编辑配置文件，或创建一个临时配置文件：

```bash
cp config.yaml config.temp.yaml
# 编辑 config.temp.yaml
./chat.exe -config config.temp.yaml
```

### Q: 配置文件在哪里？

默认在当前目录的 `config.yaml`。可以使用 `-config` 指定其他位置：

```bash
./chat.exe -config /path/to/my-config.yaml
```

### Q: 如何保护敏感配置？

1. 不要将包含真实 API Key 的配置文件提交到 Git
2. 使用文件权限保护配置文件：
   ```bash
   chmod 600 config.yaml
   ```
3. 考虑使用环境变量或密钥管理服务

## 总结

这次简化让配置管理更加规范和安全：

✅ 所有配置集中在配置文件中
�� 代码更简洁，更易维护
✅ 更适合生产环境部署
✅ 更好的安全性
✅ 避免配置来源混乱

唯一保留的命令行参数是 `-config`，用于指定配置文件路径。
