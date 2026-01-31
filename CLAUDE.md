# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Agent-Langchain is a Go-based intelligent agent system that combines LLMs, vector databases (Qdrant), and structured storage (SQLite) to create a conversational agent with memory capabilities. The system can extract and store information from conversations, retrieve relevant memories, and provide contextually aware responses.

## Build and Run Commands

### Build Commands
```bash
# Build chat CLI
go build -o chat.exe ./cmd/chat

# Build API server
go build -o main.exe ./cmd/api

# Build ingest tool
go build -o ingest.exe ./cmd/ingest
```

### Run Commands
```bash
# Run chat CLI (normal mode)
./chat.exe

# Run chat CLI with custom config
./chat.exe -config config.yaml

# View memory statistics
./chat.exe -stats -user local

# Run API server
./main.exe -config config.yaml

# Ingest knowledge from file
./ingest.exe --file test_file.txt

# Ingest knowledge from directory
./ingest.exe --dir test_dir --chunk-size 1500 --chunk-overlap 300
```

### Development Commands
```bash
# Run without building
go run ./cmd/chat -config config.yaml

# Run tests
go test ./...

# Run specific test
go test ./internal/tools -v -run TestCalculator

# Install dependencies
go mod download

# Update dependencies
go mod tidy
```

## Architecture Overview

### Core Components Flow

1. **Entry Points**: Three main entry points
   - `cmd/chat/main.go`: CLI chat interface
   - `cmd/api/main.go`: HTTP API server (Echo framework)
   - `cmd/ingest/main.go`: Knowledge base ingestion tool

2. **Orchestrator Pattern** (`internal/orchestrator/orchestrator.go`): Central coordinator that manages the entire request lifecycle
   - Retrieves memories in parallel (SQLite + Qdrant)
   - Constructs Agent input with context
   - Executes Agent
   - Asynchronously extracts and stores new memories (with smart triggering)
   - Handles both streaming and non-streaming responses

3. **Agent Layer** (`internal/agent/`): Implements conversational logic
   - `ConversationalAgent`: Handles LLM interaction, tool calling, and response generation
   - Supports streaming responses
   - Tool detection via `TOOL_CALL:` pattern in responses
   - Currently supports: calculator, speak tools

4. **Memory System** (Dual Storage):
   - **SQLite** (`internal/memory/sqlite.go`): Structured memories (key-value pairs with confidence scores, access tracking)
   - **Qdrant** (`internal/rag/qdrant.go`): Semantic memories (vector embeddings for similarity search)
   - **Window Memory** (`internal/memory/window.go`): Short-term conversation context (configurable window size)

5. **LLM Clients** (`internal/models/`):
   - `ollama.go`: Ollama API client (supports streaming)
   - `deepseek.go`: DeepSeek API client
   - Both implement `LLMClient` interface

### Memory Extraction Flow

The system uses an intelligent memory extraction system with three trigger strategies (configured in `config.yaml`):

1. **Conservative** (default): Extracts if message length exceeds threshold
2. **Keyword**: Extracts if specific keywords detected (identity, preferences, goals, skills)
3. **LLM**: Uses a small classifier model to decide if extraction is needed

Memory extraction happens **asynchronously** after response generation to avoid blocking the user. The extractor (`internal/orchestrator/memory_extractor.go`) uses a dedicated LLM call to parse conversations into structured memories.

### Configuration System

Configuration is loaded from `config.yaml` (see `config.example.yaml` for template). Key sections:
- `base`: Provider selection (ollama/deepseek), debug mode, timeout
- `llm`: Model configurations for both providers
- `storage`: Database paths and Qdrant settings
- `memory`: Window size, smart trigger settings, extractor model
- `services`: API port, RAG chunking strategy

## Key Design Patterns

### 1. Parallel Memory Retrieval
The orchestrator queries SQLite and Qdrant concurrently using goroutines and channels to minimize latency (see `retrieveMemories()` in orchestrator.go:203).

### 2. Async Memory Storage
Memory extraction runs in a background goroutine with its own context to prevent blocking the response and avoid cancellation issues (orchestrator.go:119-130).

### 3. Smart Memory Triggering
Configurable strategies to reduce token usage by 70-80% by skipping extraction for simple responses like "好的", "ok", "谢谢" (orchestrator.go:412-560).

### 4. Tool Calling Pattern
Tools are invoked via text pattern matching (`TOOL_CALL: tool_name("args")`) rather than structured API calls. The agent detects this pattern, executes the tool, and feeds results back to the LLM for natural language response.

### 5. Streaming Support
Both Ollama client and Agent support streaming responses. The system checks if the client supports streaming and falls back to non-streaming if not.

## Important Implementation Notes

### Memory System
- Memories have `owner` field: "user" for user info, "agent" for assistant info
- `AlsoVector` flag determines if memory should be stored in Qdrant (requires confidence >= 0.65)
- Access tracking: `access_count` and `last_accessed` fields track memory usage
- Fingerprinting prevents duplicate vector storage

### Context Management
- Short-term memory: Sliding window of recent conversation (default 8 rounds)
- Long-term memory: Retrieved from SQLite (structured) and Qdrant (semantic)
- Context stats calculation available in debug mode (uses `internal/utils/tokens.go`)

### API Endpoints (cmd/api)
- `/api/chat`: Non-streaming chat
- `/api/chat/stream`: Server-Sent Events streaming chat
- `/api/knowledge/*`: Knowledge base management (ingest, query, list)
- `/api/debug/*`: Debug utilities (config, logs, connection tests)

### Tool Development
To add a new tool:
1. Create tool in `internal/tools/` implementing `Execute(ctx, args) (string, error)`
2. Add case in `ConversationalAgent.executeTool()` (internal/agent/conversational.go:306)
3. Document tool usage in system prompt (cmd/chat/main.go:45-59)

## Dependencies

- **modernc.org/sqlite**: Pure Go SQLite driver
- **gopkg.in/yaml.v3**: YAML configuration parsing
- **github.com/labstack/echo/v4**: HTTP framework (API server only)
- **Standard library**: Extensive use of net/http, encoding/json, context

## Testing

Tests are located alongside source files with `_test.go` suffix. Example: `internal/tools/calculator_test.go`, `internal/utils/tokens_test.go`.
