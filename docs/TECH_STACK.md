# Technical Stack

Below is the technical stack for the project, using a hybrid Go/Python architecture with an automatically spawned Python sidecar service.

## Architecture Overview

The project follows a **Go CLI + Python Sidecar Service** pattern:

- **Go Binary**: Core CLI, file operations, git integration, AST parsing, test execution, reporting
- **Python Sidecar**: AI/ML operations, LLM routing, embeddings, agent orchestration
- **Communication**: HTTP over localhost (automatically spawned and managed by Go CLI)

```
┌─────────────────────────────────────────────────────────────┐
│                    Go CLI (dehydrate)                        │
│                                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐ │
│  │ Cobra    │  │ Config   │  │ go-git   │  │ go-tree-sitter│ │
│  │ CLI      │  │ YAML     │  │          │  │              │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────┘ │
│                                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────────────────┐│
│  │ File I/O │  │ Report   │  │ Python Client (HTTP client)  ││
│  │ Walker   │  │ Generator│  │ - analyze, refactor, embed   ││
│  └──────────┘  └──────────┘  └──────────────────────────────┘│
│                           │                                  │
└───────────────────────────┼──────────────────────────────────┘
                            │ HTTP (localhost)
                            ▼
┌─────────────────────────────────────────────────────────────┐
│              Python AI Sidecar (ai_sidecar)                  │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ FastAPI      │  │ LiteLLM      │  │ PydanticAI       │   │
│  │ Server       │  │ Router       │  │ Agent Framework  │   │
│  └──────────────┘  └──────────────┘  └──────────────────┘   │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ ChromaDB     │  │ Embeddings   │  │ LSP Client       │   │
│  │ (in-memory)  │  │ Models       │  │ (pyright, etc.)  │   │
│  └──────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## 1. Go Components (Core CLI)

### Language
- **Go 1.21+**: Provides excellent CLI tooling, single binary distribution, and native concurrency.

### CLI Framework
- **Cobra**: Industry-standard CLI framework used by Docker, Kubectl, Hugo. Provides:
  - Subcommand structure
  - Automatic help generation
  - Shell completion (bash, zsh, fish)
  - Flag parsing

### Configuration
- **Viper**: Configuration management with support for:
  - YAML config files
  - Environment variables
  - Command-line flags
  - Default values

### Code Parsing
- **go-tree-sitter**: CGO bindings for Tree-sitter. Builds concrete syntax trees (CST) for:
  - Multi-language support
  - Incremental parsing
  - Precise source location mapping

### Git Integration
- **go-git**: Pure Go git implementation for:
  - Repository state checking
  - Commit creation
  - Branch management
  - Diff generation

### File Operations
- **Standard library** + **filepath**: Directory traversal, file watching, atomic writes.

### Test Execution
- **os/exec**: Subprocess management for running:
  - Project test suites (pytest, npm test, go test)
  - Linters (ruff, eslint, golangci-lint)
  - Build verification

### Reporting
- **goldie** or custom templates: Markdown/HTML report generation.

### Python Sidecar Client
- **net/http**: HTTP client for communicating with Python sidecar.
- **Sidecar lifecycle management**: Spawn, health check, graceful shutdown.

---

## 2. Python Components (AI Sidecar)

### Language
- **Python 3.10+**: Required for AI/ML ecosystem compatibility.

### HTTP Server
- **FastAPI**: Modern async web framework for the sidecar service:
  - Automatic OpenAPI documentation
  - Async request handling
  - Type hints with Pydantic models
  - Excellent performance

### Agent Framework
- **PydanticAI**: Production-grade agent framework for:
  - Type-safe LLM interactions
  - Structured output validation
  - Tool/function calling
  - State management for multi-step workflows

### Model Router
- **LiteLLM**: Unified interface for 100+ LLM providers:
  - Local: Ollama (Qwen, Llama, DeepSeek)
  - Remote: OpenAI, Anthropic, Google, Azure
  - Automatic retries and fallbacks
  - Cost tracking

### Structured Output
- **Instructor**: Ensures LLM outputs conform to schemas:
  - JSON schema validation
  - Retry on validation failure
  - Reduces hallucination risk

### Vector Database
- **ChromaDB**: Lightweight, local vector database:
  - In-memory or persistent mode
  - Semantic code search
  - Duplicate detection via embeddings

### Embeddings
- **sentence-transformers** or **OpenAI embeddings**: Code-to-vector conversion for semantic analysis.

### LSP Integration
- **Python LSP client**: Communication with language servers:
  - pyright (Python)
  - typescript-language-server (JS/TS)
  - gopls (Go)
  - Cross-file symbol resolution

### Reliability
- **Tenacity**: Retry logic with exponential backoff for API calls.

---

## 3. Communication Protocol

### Sidecar Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/health` | GET | Service health check |
| `/analyze` | POST | AST analysis, complexity metrics |
| `/deduplicate` | POST | Semantic duplicate detection |
| `/refactor` | POST | Generate refactoring plan |
| `/embed` | POST | Generate code embeddings |
| `/shutdown` | POST | Graceful shutdown |

### Message Format

All requests/responses use JSON:

```json
// Request
{
  "session_id": "uuid",
  "files": [
    {"path": "src/main.py", "content": "..."}
  ],
  "config": {
    "model_tier": "medium",
    "language": "python"
  }
}

// Response
{
  "status": "success",
  "data": { ... },
  "error": null
}
```

### Sidecar Lifecycle

1. **Spawn**: Go CLI starts Python sidecar as subprocess
2. **Health Check**: Poll `/health` until ready (max 30s timeout)
3. **Operation**: Execute CLI commands, communicate via HTTP
4. **Shutdown**: On CLI exit, send `/shutdown` or SIGTERM

---

## 4. User Requirements

### Prerequisites
- **Go 1.21+**: For building the CLI
- **Python 3.10+**: For AI sidecar functionality
- **pip**: For installing Python dependencies

### Installation

```bash
# Build Go CLI
go build -o dehydrate ./cmd/dehydrate

# Install Python dependencies (automatic on first run, or manual)
pip install -r requirements.txt
```

### Optional (for local models)
- **Ollama**: For local LLM inference
- **GPU**: CUDA/Metal for faster local inference

---

## 5. Project Structure

```
dehydrate/
├── cmd/
│   └── dehydrate/           # CLI entrypoint
│       └── main.go
├── internal/
│   ├── cli/                 # Cobra commands
│   ├── config/              # Configuration management
│   ├── git/                 # Git operations
│   ├── parser/              # Tree-sitter integration
│   ├── walker/              # File traversal
│   ├── runner/              # Test/lint execution
│   ├── reporter/            # Report generation
│   └── sidecar/             # Python sidecar client
│       ├── client.go        # HTTP client
│       └── lifecycle.go     # Spawn/manage/shutdown
├── pkg/
│   └── models/              # Shared data structures
├── python/
│   ├── ai_sidecar/          # Python sidecar service
│   │   ├── __init__.py
│   │   ├── main.py          # FastAPI server
│   │   ├── routes/          # API endpoints
│   │   ├── agents/          # PydanticAI agents
│   │   ├── embeddings/      # Embedding generation
│   │   └── lsp/             # LSP client
│   └── requirements.txt
├── go.mod
├── go.sum
└── README.md
```

---

## 6. Key Dependencies

### Go (go.mod)

```
github.com/spf13/cobra
github.com/spf13/viper
github.com/go-git/go-git/v5
github.com/tree-sitter/go-tree-sitter
github.com/goccy/go-yaml
```

### Python (requirements.txt)

```
fastapi>=0.100.0
uvicorn>=0.23.0
pydantic>=2.0.0
pydantic-ai>=0.0.1
litellm>=1.0.0
instructor>=0.4.0
chromadb>=0.4.0
sentence-transformers>=2.2.0
tenacity>=8.0.0
```

---

## 7. Trade-offs Summary

| Aspect | Benefit | Trade-off |
|--------|---------|-----------|
| **Distribution** | Single Go binary | Requires Python installed |
| **Performance** | Fast Go core, cached Python state | Initial sidecar startup delay |
| **AI Ecosystem** | Full Python ML ecosystem | Two codebases to maintain |
| **Debugging** | Isolated services | Cross-language debugging complexity |
| **State Management** | In-memory sidecar state | Sidecar crash requires restart |
| **User Experience** | Transparent sidecar spawn | Python dependency requirement |
