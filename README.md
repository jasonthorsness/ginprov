# Ginprov

[![Release](https://img.shields.io/github/v/release/jasonthorsness/ginprov?label=release&style=flat-square)](https://github.com/jasonthorsness/ginprov/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/jasonthorsness/ginprov)](https://goreportcard.com/report/github.com/jasonthorsness/ginprov)
[![License](https://img.shields.io/github/license/jasonthorsness/ginprov?style=flat-square)](https://github.com/jasonthorsness/ginprov/blob/main/LICENSE)
[![CI](https://github.com/jasonthorsness/ginprov/actions/workflows/ci.yml/badge.svg)](https://github.com/jasonthorsness/ginprov/actions)

**Ginprov** is an AI-powered _improvisational web server_ that creates entire websites on-demand. Unlike traditional web servers that serve static files, Ginprov generates HTML pages and images in real-time using Google's Gemini AI, turning any URL path into a content prompt.

🌐 **[Try it live](https://extempore.dev)** | 📖 **[Read the story](https://www.jasonthorsness.com/28)** behind HTTP: H is for Hallucinated

## ✨ Features

- **Dynamic Content Generation**: Any URL becomes a prompt for AI-generated HTML and images
- **Real-time Creation**: Watch pages generate live with streaming progress updates  
- **Cohesive Design**: Maintains site-wide consistency through AI-generated design outlines
- **Content Persistence**: Generated content is cached and served instantly on subsequent visits
- **Safety First**: Built-in content filtering and HTML sanitization for secure operation
- **Zero Dependencies**: Generated pages contain no external resources or JavaScript
- **Multi-tenant**: Support multiple independent sites with isolated content directories

## License

Ginprov is licensed under the [MIT License](./LICENSE). If you can find a use for this, go right
ahead 🤣.

## 🚀 Quick Start

### 1. Get a Gemini API Key
Ginprov uses [Google's Gemini AI](https://developers.googleblog.com/en/experiment-with-gemini-20-flash-native-image-generation/) for content generation. Get a free API key from [Google AI Studio](https://aistudio.google.com/).

Create a `.env.local` file in your project directory:
```bash
echo "GEMINI_API_KEY=your_api_key_here" > .env.local
```

### 2. Install Ginprov

**Option A: Download binary (Linux/macOS)**
```bash
curl -Ls "https://github.com/jasonthorsness/ginprov/releases/latest/download/ginprov_\
$(uname -s | tr '[:upper:]' '[:lower:]')_\
$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')\
.tar.gz" \
| sudo tar -xz -C /usr/local/bin
ginprov --version
```

**Option B: Build from source**
```bash
git clone https://github.com/jasonthorsness/ginprov.git
cd ginprov
make
./bin/ginprov --version
```

### 3. Start Creating
```bash
# Start the server (default: http://localhost:8080)
ginprov

# Or with custom options
ginprov --host 0.0.0.0 --port 3000 --content ./my-content
```

### 4. Explore and Generate
Open your browser and visit any path - Ginprov will generate content on-demand:
- `http://localhost:8080/about` → Creates an "About" page
- `http://localhost:8080/recipes/chocolate-cake` → Generates a chocolate cake recipe
- `http://localhost:8080/blog/space-exploration.jpg` → Creates an image about space exploration

> **Note**: Generated content is cached and immutable. The homepage updates automatically as you create new pages to provide navigation links.

## 🔧 Configuration

| Flag | Short | Purpose | Default |
|------|-------|---------|---------|
| `--host` | `-H` | Host address to bind to | `localhost` |
| `--port` | `-p` | Port to listen on | `8080` |
| `--content` | | Directory to store generated content | `./content` |
| `--prompt` | | Custom prompt file describing site nature | `./default-prompt.txt` |
| `--index` | | Custom index.html template | (embedded) |
| `--notfound` | | Custom 404 page template | (embedded) |
| `--banner` | | Custom banner HTML for all pages | (none) |

**Examples:**
```bash
# Basic usage
ginprov

# Custom host and port
ginprov --host 0.0.0.0 --port 3000

# Custom content directory and prompt
ginprov --content ./my-site --prompt ./site-prompt.txt

# Full customization
ginprov -H 0.0.0.0 -p 80 --content ./prod-content --banner ./banner.html
```

## 🏗️ How It Works

Ginprov transforms URL paths into AI-generated content through a sophisticated pipeline:

### Content Generation Pipeline
1. **URL Analysis**: Incoming requests are parsed and normalized
2. **Safety Check**: Content prompts are filtered for appropriateness
3. **Context Building**: Site outline and existing links inform generation
4. **AI Generation**: Gemini creates HTML with embedded CSS or JPG images
5. **Content Sanitization**: Generated HTML is cleaned and secured
6. **Persistence**: Content is cached to disk for instant future serving

### AI Models Used
- **HTML Generation**: `gemini-2.5-flash-lite-preview-06-17` - Fast, efficient text generation
- **Image Generation**: `gemini-2.0-flash-preview-image-generation` - High-quality image creation

### Architecture Highlights
- **Worker Pool**: 100 concurrent workers handle generation requests
- **Single-Flight**: Prevents duplicate generation of the same content
- **Progressive Rendering**: Real-time updates during content creation
- **Content Safety**: Built-in filtering and HTML sanitization
- **Multi-tenant**: Isolated content directories per URL prefix

## 🛠️ Building from Source

**Requirements:**
- Go 1.24.3 or later
- Google Gemini API key

**Build commands:**
```bash
make          # Build binary to ./bin/ginprov
make test     # Run tests  
make lint     # Run linting
make clean    # Clean build artifacts
```
