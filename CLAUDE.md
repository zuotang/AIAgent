# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Agent-Langchain is a Go-based intelligent agent system that combines LLMs (via Ollama or DeepSeek API), vector databases (Qdrant), and structured storage (SQLite) to create a conversational AI with memory capabilities. The system extracts key information from conversations and stores it in both structured (SQLite) and semantic (Qdrant) memory stores for personalized, context-aware responses.

## Configuration Management

The project uses YAML configuration files for managing settings. See CONFIG.md for detailed configuration guide.

### Quick Start

1. Copy example config: `cp config.example.yaml config.yaml`
2. Edit `config.yaml` to set your preferences
3. Run: `./chat.exe`

### Configuration Files

- `config.yaml`: Main configuration file (gitignored, contains sensitive data)
- `config.example.yaml`: Example configuration template
- `config.deepseek.example.yaml`: Example for DeepSeek API usage
- `internal/config/config.go`: Configuration structure and loading logic

## Build and Run Commands

### Build
```bash
go build -o chat.exe ./cmd/chat
```

### Run
```bash
# Use default config.yaml
./chat.exe

# Use specific config file
./chat.exe -config my-config.yaml
```

### Run Without Building
```bash
go run ./cmd/chat
```

### Command-Line Options

Only one command-line flag is supported:

- `-config`: Configuration file path (default: config.yaml)

**All other settings must be configured in the config file.**

## Dependencies

### External Services
1. **Ollama**: LLM inference server
   - Install from https://ollama.com/download
   - Start with `ollama serve`
   - Pull required models: `ollama pull gemma3:12b` and `ollama pull nomic-embed-text`

2. **Qdrant**: Vector database for semantic memory
   - Start with Docker: `docker-compose up -d`
   - The docker-compose.yml configures Qdrant on ports 6333 (HTTP) and 6334 (gRPC)

### Go Dependencies
- `modernc.org/sqlite`: Pure Go SQLite driver
- `github.com/tmc/langchaingo`: LangChain Go library
- Standard library packages for HTTP, JSON, database operations

## Architecture

### Memory System (Dual-Store Architecture)
The system uses a dual-memory architecture to balance structured queries and semantic search:

1. **SQLite (Structured Memory)**: Stores extracted facts as key-value pairs with metadata
   - Schema: `(user_id, mtype, mkey, mvalue, confidence, owner, updated_at)`
   - Unique constraint on `(user_id, mtype, mkey, owner)`
   - Memory types: identity, preference, goal, tool, constraint, fact, activity, duration
   - Owner field distinguishes between user and agent memories

2. **Qdrant (Semantic Memory)**: Stores vector embeddings for similarity-based retrieval
   - Each memory point includes: `user_id`, `text`, `timestamp`
   - Uses cosine distance for similarity search
   - Filtered by user_id to maintain user isolation

### Memory Extraction Pipeline
The system uses a dedicated "memory extractor" LLM call to analyze conversations:

1. **Input**: Recent conversation window + current user input + assistant response
2. **Extraction**: LLM identifies stable, reusable information (preferences, identity, goals, facts)
3. **Normalization**: Keys are normalized (e.g., "user_name" → "name"), sensitive data filtered
4. **Validation**: Checks confidence threshold (≥0.65), valid types/owners, key format
5. **Storage**: Writes to both SQLite (structured) and Qdrant (semantic, if `also_vector=true`)

### Conversation Flow
1. User provides UID and input
2. System retrieves structured memory from SQLite (last 30 entries)
3. System performs semantic search in Qdrant (top-K similar memories)
4. Combines short-term window + structured memory + semantic memory into prompt
5. LLM generates response
6. Memory extractor analyzes the conversation turn
7. Extracted memories are sanitized and stored
8. Short-term window is updated (sliding window of last N turns)

### Key Components

**internal/config/config.go**: Configuration management
- `Load(path)`: Load configuration from YAML file
- `Validate()`: Validate configuration settings
- `setDefaults()`: Set default values for missing config options
- Supports both file-based and command-line configuration

**internal/models/ollama.go**: Ollama API client
- `Chat(ctx, messages, model)`: Send chat completion request
- `Embed(ctx, text)`: Generate text embeddings
- Supports debug mode to log API requests/responses
- Implements `LLMClient` and `EmbedClient` interfaces

**internal/models/deepseek.go**: DeepSeek API client
- `Chat(ctx, messages, model)`: Send chat completion request to DeepSeek API
- Uses OpenAI-compatible API format
- Implements `LLMClient` interface
- Note: Does not support embeddings (use Ollama for embeddings)

**internal/memory/sqlite.go**: SQLite storage layer
- `UpsertExtractedMemories(ctx, userID, memories)`: Insert/update memories
- `RenderStructuredMemory(ctx, userID, limit)`: Format memories as text for prompts

**internal/rag/qdrant.go**: Qdrant vector store client
- `EnsureCollection(ctx, dim)`: Create collection if not exists
- `SimilaritySearch(ctx, userID, query, topK)`: Retrieve similar memories
- `UpsertTexts(ctx, userID, texts)`: Store text embeddings
- Includes retry logic for network resilience

**cmd/chat/main.go**: Main application
- Implements conversation loop with graceful shutdown (Ctrl+C)
- Memory extraction with configurable extractor model
- Sanitization functions to prevent sensitive data storage
- Fingerprinting for deduplication

### Owner Field Semantics
The `owner` field is critical for distinguishing memory attribution:
- `owner="user"`: Facts about the user extracted from user input
- `owner="agent"`: Facts about the assistant/agent (name, personality, capabilities)
- The system prompt explicitly instructs the LLM not to confuse these

### Memory Sanitization
The system filters out:
- Sensitive information (passwords, tokens, ID cards, addresses, phone numbers, emails)
- Low confidence memories (< 0.65)
- Invalid types/owners
- Malformed keys (must match `^[a-z0-9_]+$`)
- Overly long values (> 512 chars)
- Duplicate memories (via SHA1 fingerprinting)

## Important Notes

- The `cmd/ingest/main.go` file is currently commented out and not functional
- The system uses a "companion chat" persona focused on casual conversation and emotional support
- Debug mode (`-debug`) provides detailed logging of LLM interactions, memory extraction, and storage operations
- All Ollama and Qdrant calls include configurable timeouts and context cancellation support
- The short-term memory window prevents context from growing unbounded
- **DeepSeek Integration**: The system supports both Ollama (local) and DeepSeek (cloud API) as LLM providers
  - DeepSeek is used for chat and memory extraction
  - Ollama is still required for embeddings (DeepSeek doesn't support embeddings)
  - Use `-provider deepseek -deepseek-key YOUR_KEY` to enable DeepSeek
  - See DEEPSEEK.md for detailed usage instructions
