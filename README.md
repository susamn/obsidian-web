# Obsidian Web

A web-based viewer and editor for Obsidian vaults with support for multiple storage backends (local, S3, MinIO) and LLM integration.

## Features

- 📁 Multi-vault support (Local, S3, MinIO)
- 🔗 Full wikilink support with graph visualization
- 🤖 LLM integration (OpenAI, Anthropic, Ollama, Custom)
- 📝 Markdown rendering with Obsidian-specific features
- 🔍 Search and tag support
- 🌓 Dark mode support
- 🚀 Lazy loading for performance
- 🔄 Conflict detection and resolution

## Architecture

### Backend (Go)
- **cmd/server**: Application entry point
- **internal/vault**: Storage abstraction layer
- **internal/metadata**: Metadata extraction from markdown files
- **internal/connector**: Knowledge graph builder
- **internal/llm**: LLM provider abstraction
- **internal/orchestrator**: REST API layer

**Key Libraries:**
- chi/v5: HTTP router
- logrus: Structured logging
- viper: Configuration management
- AWS SDK v2: S3 support
- Minio SDK: MinIO support

### Frontend (Vue.js)
- **views**: Page components
- **components**: Reusable UI components
- **services**: API client services
- **stores**: Pinia state management
- **router**: Vue Router configuration

**Key Libraries:**
- Vue 3 + Vue Router + Pinia
- Element Plus: UI framework
- Tailwind CSS: Utility-first CSS
- markdown-it: Markdown parsing
- D3.js + Cytoscape: Graph visualization
- lodash-es: Utility functions
- dayjs: Date/time handling
- axios: HTTP client

## Prerequisites

- Go 1.22 or higher
- Node.js 18 or higher
- npm or yarn

## Quick Start

### Installation

```bash
# Install all dependencies (backend + frontend)
make install
```

### Development

```bash
# Terminal 1: Start backend (default port 8080)
make dev-backend

# Terminal 2: Start frontend (default port 3000)
make dev-frontend
```

### Building

```bash
# Build both backend and frontend
make build
```

### Testing

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage
```

## Docker Deployment

```bash
# Build Docker images
make docker-build

# Start containers
make docker-up

# View logs
make docker-logs

# Stop containers
make docker-down
```

## Configuration

See `config/config.example.yaml` for configuration options.

Key configuration areas:
- Server settings (host, port, timeout)
- Vault connections (local, S3, MinIO)
- LLM providers (OpenAI, Anthropic, Ollama, Custom)
- Logging, caching, CORS, rate limiting
- Conflict resolution strategies

## Project Guidelines

See `.progress` files in each directory for implementation guidelines and TODO lists.

Key principles:
- Write tests for all changes
- Check for code duplication
- Consider security implications
- Optimize for performance
- Follow accessibility guidelines

## API Documentation

The REST API is available at `http://localhost:8080/api/v1/`

Key endpoints:
- `GET /api/v1/vaults` - List vaults
- `GET /api/v1/vault/:id/note/:path` - Get note
- `POST /api/v1/vault/:id/note` - Create note
- `PUT /api/v1/vault/:id/note/:path` - Update note
- `GET /api/v1/vault/:id/graph` - Get graph data
- `POST /api/v1/llm/chat` - Chat with LLM

## Development Commands

```bash
make help              # Show all available commands
make install           # Install dependencies
make build             # Build project
make test              # Run tests
make test-coverage     # Run tests with coverage
make lint              # Run linters
make fmt               # Format code
make clean             # Clean build artifacts
make tidy              # Tidy Go modules
```

## License

MIT

## Contributing

1. Check `.progress` files for TODOs
2. Write tests for your changes
3. Run `make test` and `make lint`
4. Submit pull request
