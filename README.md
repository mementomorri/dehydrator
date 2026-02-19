# DeHydrator

**Semantic Code Compression Engine**

Autonomously refactor codebases to reduce complexity and eliminate duplication while maintaining 100% functional parity.

## Install

### Download Binary

```bash
curl -sSL https://github.com/alexkarsten/dehydrate/releases/latest/download/dehydrate-linux-amd64 -o dehydrate
chmod +x dehydrate
sudo mv dehydrate /usr/local/bin/
```

### Build from Source

Requires Go 1.21+ and Python 3.10+.

```bash
git clone https://github.com/alexkarsten/dehydrate.git
cd dehydrate
pip install -r python/requirements.txt
go build -o dehydrate ./cmd/dehydrate
```

## Usage

```bash
dehydrate analyze .              # Scan for complexity hotspots and duplicates
dehydrate deduplicate .          # Find and eliminate code duplication
dehydrate idiomatize .           # Transform code to idiomatic patterns
dehydrate pattern factory .      # Apply a design pattern
```

### Flags

- `--yes` - Skip approval prompts, apply changes automatically
- `--commit` - Commit changes to git after successful refactoring
- `--report` - Generate a detailed report

## Features

- **Cross-file deduplication** - Detect semantically similar code using embeddings
- **Idiomatic transformation** - Convert to Pythonic, modern JS, Go idioms
- **Design pattern injection** - Apply Factory, Strategy, Observer patterns
- **Complexity detection** - Identify cyclomatic/cognitive hotspots
- **Git-integrated safety** - Checkpoints, rollback on test failure
- **Local-first AI** - Use Ollama locally or cloud providers via LiteLLM

## Documentation

- [Architecture Overview](docs/TECH_STACK.md)
- [Design Rationale](docs/DESIGN.md)
- [Testing Philosophy](docs/TEST_RULES.md)
- [Contributing Guidelines](AGENTS.md)

## License

[MIT](LICENSE) © 2026 Alex Karsten
