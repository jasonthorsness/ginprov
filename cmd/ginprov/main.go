//nolint:forbidigo
package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jasonthorsness/ginprov/gemini"
	"github.com/jasonthorsness/ginprov/server"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"golang.org/x/net/html"
)

//go:embed index.html notfound.html banner.html safety.html favicon.ico robots.txt
var staticFiles embed.FS

func findHeadAndBody(doc *html.Node) (*html.Node, *html.Node) {
	var head, body *html.Node

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "head":
				head = n
			case "body":
				body = n
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)

	return head, body
}

func addSocialCardMeta(head *html.Node, prefix, baseURL string) {
	if head == nil {
		return
	}

	var socialCardURL string
	if baseURL != "" {
		// Use absolute URL with provided base URL
		socialCardURL = strings.TrimSuffix(baseURL, "/") + "/" + prefix + "/colorful-social-card.jpg"
	} else {
		// Use relative URL as fallback
		socialCardURL = "/" + prefix + "/colorful-social-card.jpg"
	}

	metaTags := []*html.Node{
		// Standard meta description
		{
			Type: html.ElementNode,
			Data: "meta",
			Attr: []html.Attribute{
				{Key: "name", Val: "description"},
				{Key: "content", Val: "AI-generated content for " + prefix},
			},
		},
		// Open Graph
		{
			Type: html.ElementNode,
			Data: "meta",
			Attr: []html.Attribute{
				{Key: "property", Val: "og:image"},
				{Key: "content", Val: socialCardURL},
			},
		},
		{
			Type: html.ElementNode,
			Data: "meta",
			Attr: []html.Attribute{
				{Key: "property", Val: "og:title"},
				{Key: "content", Val: prefix},
			},
		},
		{
			Type: html.ElementNode,
			Data: "meta",
			Attr: []html.Attribute{
				{Key: "property", Val: "og:description"},
				{Key: "content", Val: prefix},
			},
		},
		{
			Type: html.ElementNode,
			Data: "meta",
			Attr: []html.Attribute{
				{Key: "property", Val: "og:url"},
				{Key: "content", Val: strings.TrimSuffix(baseURL, "/") + "/" + prefix + "/"},
			},
		},

		// Twitter Cards
		{
			Type: html.ElementNode,
			Data: "meta",
			Attr: []html.Attribute{
				{Key: "name", Val: "twitter:card"},
				{Key: "content", Val: "summary_large_image"},
			},
		},
		{
			Type: html.ElementNode,
			Data: "meta",
			Attr: []html.Attribute{
				{Key: "name", Val: "twitter:title"},
				{Key: "content", Val: prefix},
			},
		},
		{
			Type: html.ElementNode,
			Data: "meta",
			Attr: []html.Attribute{
				{Key: "name", Val: "twitter:description"},
				{Key: "content", Val: prefix},
			},
		},
		{
			Type: html.ElementNode,
			Data: "meta",
			Attr: []html.Attribute{
				{Key: "name", Val: "twitter:image"},
				{Key: "content", Val: socialCardURL},
			},
		},
	}

	for _, tag := range metaTags {
		head.AppendChild(tag)
	}
}

func insertBannerIframe(body *html.Node) error {
	if body == nil {
		return nil
	}

	// Create iframe element with responsive height
	iframe := &html.Node{
		Type: html.ElementNode,
		Data: "iframe",
		Attr: []html.Attribute{
			{Key: "src", Val: "/banner.html", Namespace: ""},
			{Key: "title", Val: "AI-generatedwarning banner and header", Namespace: ""},
			{
				Key: "style",
				Val: "position: fixed !important; top: 0 !important; left: 0 !important; " +
					"right: 0 !important; z-index: 999999 !important; border: none !important; " +
					"height: 80px !important; width: 100% !important;",
				Namespace: "",
			},
			{Key: "scrolling", Val: "no", Namespace: ""},
		},
	}

	// Create spacer div with responsive height
	spacer := &html.Node{
		Type: html.ElementNode,
		Data: "div",
		Attr: []html.Attribute{
			{
				Key: "style",
				Val: "height: 80px !important; margin: 0 !important; " +
					"padding: 0 !important; box-sizing: border-box !important;",
				Namespace: "",
			},
		},
	}

	// Insert iframe and spacer at the beginning of body
	if body.FirstChild != nil {
		body.InsertBefore(spacer, body.FirstChild)
		body.InsertBefore(iframe, spacer)
	} else {
		body.AppendChild(iframe)
		body.AppendChild(spacer)
	}

	return nil
}

func createDefaultTransformer(prefix, baseURL string) server.HTMLTransformer {
	return func(doc *html.Node, urls map[string]struct{}) error {
		head, body := findHeadAndBody(doc)
		addSocialCardMeta(head, prefix, baseURL)

		urls["colorful-social-card.jpg"] = struct{}{}

		return insertBannerIframe(body)
	}
}

type Config struct {
	host       string
	contentDir string
	baseURL    string
	port       int
}

func createRootCmd() *cobra.Command {
	const defaultPort = 8080

	config := &Config{
		port:       defaultPort,
		host:       "localhost",
		baseURL:    "",
		contentDir: "",
	}

	rootCmd := &cobra.Command{
		Use:   "ginprov",
		Short: "✨ An Improvisational Web Server ✨",
		Long:  "ginprov generates web pages and images based on their URL paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(cmd, args, config)
		},
	}

	rootCmd.Flags().StringVarP(&config.host, "host", "H", "localhost", "Host address to listen on")
	rootCmd.Flags().IntVarP(&config.port, "port", "p", defaultPort, "Port to listen on")
	rootCmd.Flags().StringVar(&config.baseURL, "base-url", "",
		"Base URL for absolute links in social cards (e.g., https://example.com)")

	rootCmd.Flags().StringVar(
		&config.contentDir,
		"content",
		"",
		"The path to the location for generated HTML and images")

	return rootCmd
}

func main() {
	rootCmd := createRootCmd()

	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}

const maxPrefixLength = 40

//nolint:cyclop
func createHTTPHandler(
	config *Config,
	prefixRe *regexp.Regexp,
	root *os.Root,
	rootPath string,
	gen *gemini.Client,
	workerPool *server.WorkerPool,
	servers map[string]*server.Server,
	mu *sync.Mutex,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimLeft(r.URL.Path, "/")

		if path == "" || path == "index.html" {
			handleStaticFile(w, "index.html", "text/html; charset=utf-8", root)
			return
		}

		if path == "banner.html" {
			handleStaticFile(w, "banner.html", "text/html; charset=utf-8", root)
			return
		}

		if path == "favicon.ico" {
			handleStaticFile(w, "favicon.ico", "image/x-icon", root)
			return
		}

		if path == "robots.txt" {
			handleStaticFile(w, "robots.txt", "text/plain", root)
			return
		}

		if path == "api/sites" {
			handleSitesAPI(w, root)
			return
		}

		raw, path, ok := strings.Cut(path, "/")

		prefix := strings.ToLower(raw)
		prefix = prefixRe.ReplaceAllString(prefix, "-")
		prefix = strings.Trim(prefix, "-")

		if prefix != raw || len(prefix) > maxPrefixLength {
			handleStaticFile(w, "notfound.html", "text/html; charset=utf-8", root)
			return
		}

		if !ok {
			http.Redirect(w, r, "/"+prefix+"/", http.StatusMovedPermanently)
			return
		}

		mu.Lock()
		defer mu.Unlock()

		s, ok := servers[prefix]
		if !ok {
			var err error

			s, err = newServer(root, filepath.Join(rootPath, prefix), gen, workerPool, prefix, config)
			if err != nil {
				http.Error(
					w,
					fmt.Sprintf("Failed to create server for prefix %s: %v", prefix, err),
					http.StatusInternalServerError)

				return
			}

			servers[prefix] = s
		}

		r.URL.Path = path
		s.Get().ServeHTTP(w, r)
	}
}

func runServer(_ *cobra.Command, _ []string, config *Config) error {
	prefixRe := regexp.MustCompile(`[^a-z0-9]`)

	println("✨ An Improvisational Web Server ✨")

	_ = godotenv.Load(".env.local")

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		println("❌ GEMINI_API_KEY not set!")
		println("Please set the GEMINI_API_KEY environment variable or create a .env.local file with the key.")
		println("You can obtain an API key FREE from https://aistudio.google.com/apikey.")
		os.Exit(1)
	}

	ctx := context.Background()

	gen, err := gemini.New(ctx, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create gemini client: %w", err)
	}

	contentDir := config.contentDir
	if contentDir == "" {
		defaultContentDir, cacheErr := os.UserCacheDir()
		if cacheErr != nil {
			defaultContentDir = os.TempDir()
		}

		contentDir = filepath.Join(defaultContentDir, "ginprov")

		const dirPerm = 0o750

		err = os.MkdirAll(contentDir, dirPerm)
		if err != nil {
			return fmt.Errorf("failed to create default content directory %s: %w", contentDir, err)
		}
	}

	root, err := os.OpenRoot(contentDir)
	if err != nil {
		return fmt.Errorf("failed to open content directory: %w", err)
	}

	servers := make(map[string]*server.Server)
	var mu sync.Mutex

	const numWorkers = 100
	const workChannelCapacityPerWorker = 10

	workerPool := server.NewWorkerPool(numWorkers, numWorkers*workChannelCapacityPerWorker)

	handler := createHTTPHandler(config, prefixRe, root, contentDir, gen, workerPool, servers, &mu)
	http.HandleFunc("/", handler)

	addr := fmt.Sprintf("%s:%d", config.host, config.port)

	const readHeaderTimeout = 3 * time.Second

	s := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	println("Serving from " + contentDir)
	println("Listening on http://" + addr)

	err = s.ListenAndServe()
	if err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

func newServer(
	root *os.Root,
	rootPath string,
	gen *gemini.Client,
	workerPool *server.WorkerPool,
	prefix string,
	config *Config,
) (*server.Server, error) {
	rr, err := root.OpenRoot(prefix)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to open root directory %s: %w", prefix, err)
		}

		const rootPerms = 0o755

		err = root.Mkdir(prefix, rootPerms)
		if err != nil {
			return nil, fmt.Errorf("failed to create content directory %s: %w", prefix, err)
		}

		rr, err = root.OpenRoot(prefix)
		if err != nil {
			return nil, fmt.Errorf("failed to open root directory %s: %w", prefix, err)
		}
	}

	prompter := server.NewPrompter(gen, prefix, rr, rootPath)

	transformer := createDefaultTransformer(prefix, config.baseURL)
	site := server.NewSite(gen, prompter, rr, rootPath, transformer)

	var unsafeHandler server.HandleFunc = func(w http.ResponseWriter) error {
		handleStaticFile(w, "safety.html", "text/html; charset=utf-8", root)
		return nil
	}

	return server.NewServer(site, workerPool, slog.Default(), &server.DefaultProgressWriter{}, unsafeHandler), nil
}

func handleStaticFile(w http.ResponseWriter, filename, contentType string, root *os.Root) {
	content, err := getStaticFile(filename, root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)

	var cacheControl string
	if filename == "index.html" {
		cacheControl = "public, max-age=10"
	} else {
		cacheControl = "public, max-age=3600"
	}

	w.Header().Set("Cache-Control", cacheControl)

	_, err = w.Write(content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type Site struct {
	CreationTime time.Time `json:"-"` // Don't include in JSON response
	Slug         string    `json:"slug"`
	ImagePath    string    `json:"imagePath"`
}

func handleSitesAPI(w http.ResponseWriter, root *os.Root) {
	f, err := root.Open(".")
	if err != nil {
		http.Error(w, "Failed to open content directory", http.StatusInternalServerError)
		return
	}

	dirs, err := f.ReadDir(0)
	if err != nil {
		http.Error(w, "Failed to read content directory", http.StatusInternalServerError)
		return
	}

	sites := make([]Site, 0, len(dirs))

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		slug := dir.Name()
		imagePath := "/" + slug + "/colorful-social-card.jpg"

		var stat os.FileInfo

		stat, err = root.Stat(slug + "/colorful-social-card.jpg")
		if err != nil {
			continue
		}

		sites = append(sites, Site{
			Slug:         slug,
			ImagePath:    imagePath,
			CreationTime: stat.ModTime(),
		})
	}

	sort.Slice(sites, func(i, j int) bool {
		return sites[i].CreationTime.After(sites[j].CreationTime)
	})

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=10")

	err = json.NewEncoder(w).Encode(sites)
	if err != nil {
		http.Error(w, "Failed to encode sites", http.StatusInternalServerError)
		return
	}
}

func getStaticFile(filename string, root *os.Root) ([]byte, error) {
	// First try to read from content directory
	if root != nil {
		file, err := root.Open(filename)
		if err == nil {
			defer func() {
				_ = file.Close() // Ignore error in defer
			}()

			content, err := io.ReadAll(file)
			if err == nil {
				return content, nil
			}
		}
		// Ignore error and fall back to embedded
	}

	// Fall back to embedded file
	content, err := staticFiles.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded file %s: %w", filename, err)
	}

	return content, nil
}
