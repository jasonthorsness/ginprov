package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/jasonthorsness/ginprov/gemini"
	"github.com/jasonthorsness/ginprov/sanitize"
	"golang.org/x/net/html"
)

type (
	HandleFunc      func(http.ResponseWriter) error
	GenerateFunc    func(context.Context, func(string)) HandleFunc
	HTMLTransformer func(*html.Node, map[string]struct{}) error
)

var (
	ErrNotFound       = errors.New("not found")
	ErrUnexpectedSize = errors.New("unexpected size")
	ErrInvalidSlug    = errors.New("invalid slug")
)

const (
	ContentTypeHTML = "text/html; charset=utf-8"
	ContentTypeJPG  = "image/jpeg"
)

const (
	IndexSlug    = "index.html"
	NotFoundSlug = "not-found.html"
)

const (
	LinksTXT = "links.txt"
)

const (
	ExtensionHTML = ".html"
	ExtensionJPG  = ".jpg"
)

type Site interface {
	Handle(slug string) (HandleFunc, GenerateFunc, error)
}

func NewSite(
	gemini *gemini.Client,
	prompter Prompter,
	root *os.Root,
	rootPath string,
	transformer HTMLTransformer,
) Site {
	return &defaultSite{gemini, nil, prompter, root, rootPath, transformer, "", sync.Mutex{}, false}
}

type resource struct {
	size int64
	mu   sync.Mutex
}

type defaultSite struct {
	gemini      *gemini.Client
	resources   map[string]*resource
	prompter    Prompter
	root        *os.Root
	rootPath    string
	transformer HTMLTransformer
	links       string
	mu          sync.Mutex
	unsafe      bool
}

func (s *defaultSite) Handle(slug string) (HandleFunc, GenerateFunc, error) {
	if s.unsafe {
		return nil, nil, ErrUnsafe
	}

	r, err := s.getResource(slug)
	if err != nil {
		return nil, nil, err
	}

	if r.size > 0 {
		return s.handleFile(slug, r.size), nil, nil
	}

	return s.handleGenerate(slug)
}

func (s *defaultSite) handleGenerate(slug string) (HandleFunc, GenerateFunc, error) {
	handleFunc := func(w http.ResponseWriter) error {
		w.Header().Set("Content-Type", contentTypeForSlug(slug))
		w.WriteHeader(http.StatusAccepted)

		flusher, ok := w.(http.Flusher)
		if ok {
			flusher.Flush()
		}

		return nil
	}

	generateFunc := func(ctx context.Context, progress func(string)) HandleFunc {
		r, err := s.getResource(slug)
		if err != nil {
			return func(w http.ResponseWriter) error {
				http.Error(w, fmt.Sprintf("failed to initResources %s: %v", slug, err), http.StatusInternalServerError)
				return nil
			}
		}

		r.mu.Lock()
		defer r.mu.Unlock()

		if r.size > 0 {
			return s.handleFile(slug, r.size)
		}

		v, err := s.generate(ctx, slug, progress)
		if err != nil {
			if errors.Is(err, ErrUnsafe) {
				s.unsafe = true

				return func(_ http.ResponseWriter) error {
					return err
				}
			}

			return func(w http.ResponseWriter) error {
				http.Error(w, fmt.Sprintf("failed to generate %s: %v", slug, err), http.StatusInternalServerError)
				return nil
			}
		}

		err = writeFileAtomic(s.root, s.rootPath, slug, v)
		if err != nil {
			return func(w http.ResponseWriter) error {
				http.Error(
					w,
					fmt.Sprintf("failed to write generated file: %s %d", slug, len(v)),
					http.StatusInternalServerError)

				return nil
			}
		}

		r.size = int64(len(v))

		return func(w http.ResponseWriter) error {
			w.Header().Set("Content-Length", strconv.FormatInt(r.size, 10))
			w.Header().Set("Content-Type", contentTypeForSlug(slug))
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

			_, err := w.Write(v)
			if err != nil {
				return fmt.Errorf("failed to write response: %w", err)
			}

			return nil
		}
	}

	return handleFunc, generateFunc, nil
}

func (s *defaultSite) generate(ctx context.Context, slug string, progress func(string)) ([]byte, error) {
	var v []byte

	progress(fmt.Sprintf("Generating %s...\n", slug))

	s.mu.Lock()
	links := s.links
	s.mu.Unlock()

	prompt, err := s.prompter.GetPromptForSlug(ctx, slug, links, progress)
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt for %s: %w", slug, err)
	}

	switch extensionForSlug(slug) {
	case ExtensionHTML:
		v, err = s.generateHTML(ctx, prompt, progress)
	case ExtensionJPG:
		v, err = s.generateJPG(ctx, prompt, progress)
	default:
		panic(errorInvalidSlug(slug))
	}

	if err != nil {
		return nil, err
	}

	if len(v) == 0 {
		return nil, fmt.Errorf("%w: %s %d", ErrUnexpectedSize, slug, len(v))
	}

	return v, nil
}

func (s *defaultSite) getResource(slug string) (*resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.resources == nil {
		err := s.initResources()
		if err != nil {
			return nil, err
		}
	}

	r, ok := s.resources[slug]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, slug)
	}

	return r, nil
}

func (s *defaultSite) initResources() error {
	s.resources = make(map[string]*resource, 2)

	s.resources[IndexSlug] = &resource{0, sync.Mutex{}}
	s.resources[NotFoundSlug] = &resource{0, sync.Mutex{}}

	f, err := s.root.Open(LinksTXT)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to open %s: %w", LinksTXT, err)
		}

		return nil
	}

	content, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", LinksTXT, err)
	}

	lines := strings.Split(string(content), "\n")

	lines = append(lines, IndexSlug, NotFoundSlug)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var size int64

		stat, err := s.root.Stat(line)
		if err == nil {
			size = stat.Size()
		}

		s.resources[line] = &resource{size, sync.Mutex{}}
	}

	return nil
}

func (s *defaultSite) handleFile(slug string, size int64) func(http.ResponseWriter) error {
	return func(w http.ResponseWriter) error {
		f, err := s.root.Open(slug)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", slug, err)
		}

		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		w.Header().Set("Content-Type", contentTypeForSlug(slug))
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

		n, err := io.Copy(w, f)
		if err != nil {
			return fmt.Errorf("failed to copy file %s: %w", slug, err)
		}

		if n != size {
			return fmt.Errorf("%w: wrote %d bytes, expected %d bytes", ErrUnexpectedSize, n, size)
		}

		return nil
	}
}

func (s *defaultSite) generateHTML(ctx context.Context, prompt string, progress func(string)) ([]byte, error) {
	doc, err := s.gemini.HTML(ctx, prompt, progress)
	if err != nil {
		return nil, fmt.Errorf("provider.HTML failed: %w", err)
	}

	urls := make(map[string]struct{})

	err = sanitize.HTMLSanitizeAndExtractUrls(doc, urls, sanitizeURL)
	if err != nil {
		return nil, err
	}

	if s.transformer != nil {
		err = s.transformer(doc, urls)
		if err != nil {
			return nil, fmt.Errorf("transformer failed: %w", err)
		}
	}

	var sb strings.Builder

	for u := range urls {
		_, ok := s.resources[u]
		if !ok {
			sb.WriteString(u)
			sb.WriteString("\n")

			s.resources[u] = &resource{0, sync.Mutex{}}
		}
	}

	if sb.Len() > 0 {
		err = appendContents(s.root, LinksTXT, []byte(sb.String()))
		if err != nil {
			return nil, err
		}
	}

	buf := bytes.Buffer{}

	err = html.Render(&buf, doc)
	if err != nil {
		return nil, fmt.Errorf("failed to render HTML: %w", err)
	}

	v := buf.Bytes()

	return v, nil
}

func (s *defaultSite) generateJPG(ctx context.Context, prompt string, progress func(string)) ([]byte, error) {
	var raw []byte
	var err error

	for attempt := range 3 {
		raw, err = s.gemini.PNG(ctx, prompt, progress)
		if err == nil {
			break
		}

		if attempt < 2 {
			progress(fmt.Sprintf("PNG generation failed (attempt %d/3), retrying...\n", attempt+1))
		}
	}

	if err != nil {
		return nil, fmt.Errorf("provider.PNG failed after 3 attempts: %w", err)
	}

	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %w", err)
	}

	var buf bytes.Buffer

	err = jpeg.Encode(&buf, img, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	v := buf.Bytes()

	return v, nil
}

func extensionForSlug(slug string) string {
	v := slug

	idx := strings.LastIndexByte(v, '.')
	if idx < 0 {
		panic(errorInvalidSlug(slug))
	}

	return v[idx:]
}

func contentTypeForSlug(slug string) string {
	switch extensionForSlug(slug) {
	case ExtensionHTML:
		return ContentTypeHTML
	case ExtensionJPG:
		return ContentTypeJPG
	default:
		panic(errorInvalidSlug(slug))
	}
}

func errorInvalidSlug(slug string) error {
	return fmt.Errorf("%w: %s", ErrInvalidSlug, slug)
}

var sanitizeRe = regexp.MustCompile(`[^a-z0-9]`)

func sanitizeURL(v string) string {
	u, err := url.Parse(v)
	if err != nil {
		return "data:"
	}

	if u.Path == "" {
		return "index.html"
	}

	path := strings.ToLower(u.Path)
	ext := ""

	idx := strings.LastIndexByte(path, '.')
	if idx >= 0 {
		ext = path[idx:]
		path = path[:idx]
	}

	if u.RawQuery != "" {
		path += "?" + strings.ToLower(u.RawQuery)
	}

	safe := path
	safe = sanitizeRe.ReplaceAllString(safe, "-")
	safe = strings.Trim(safe, "-")

	if len(safe) == 0 {
		return "index.html"
	}

	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg":
		safe += ExtensionJPG
	case "", ".html", ".htm":
		safe += ExtensionHTML
	default:
		return "data:"
	}

	return safe
}
