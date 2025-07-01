# Ginprov

[![Release](https://img.shields.io/github/v/release/jasonthorsness/ginprov?label=release&style=flat-square)](https://github.com/jasonthorsness/ginprov/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/jasonthorsness/ginprov)](https://goreportcard.com/report/github.com/jasonthorsness/ginprov)
[![License](https://img.shields.io/github/license/jasonthorsness/ginprov?style=flat-square)](https://github.com/jasonthorsness/ginprov/blob/main/LICENSE)
[![CI](https://github.com/jasonthorsness/ginprov/actions/workflows/ci.yml/badge.svg)](https://github.com/jasonthorsness/ginprov/actions)

**Ginprov** is an _improvisational web server_. Unlike traditional web servers that serve static
files, Ginprov generates HTML pages and images in real-time using Google's Gemini AI, turning any
URL path into a content prompt.

🌐 [Try it Live](https://ginprov.com)

📖 [Read the Blog](https://www.jasonthorsness.com/28)

## 🚀 Quick Start

Feel guilty for using my credits on [ginprov.com](https://ginprov.com)? Run it locally for FREE!

### 1. Get a Gemini API Key

Ginprov uses
[Google's Gemini AI](https://developers.googleblog.com/en/experiment-with-gemini-20-flash-native-image-generation/)
for content generation. Get a free API key from [Google AI Studio](https://aistudio.google.com/).

Create a `.env.local` file in your project directory:

```bash
echo "GEMINI_API_KEY=your_api_key_here" > .env.local
```

### 2. Get Ginprov (Mac/Linux)

```bash
curl -Ls "https://github.com/jasonthorsness/ginprov/releases/latest/download/ginprov_\
$(uname -s | tr '[:upper:]' '[:lower:]')_\
$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')\
.tar.gz" \
| sudo tar -xz -C /usr/local/bin
```

### 3. Start Creating

```bash
ginprov
```

## License

Ginprov is licensed under the [MIT License](./LICENSE). If you can find a use for this, go right
ahead 🤣.

## Contact

File an issue or find me at [@jasonthorsness](https://x.com/jasonthorsness) on X.
