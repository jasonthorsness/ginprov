package sanitize

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestHTMLSanitizeUrls(t *testing.T) {
	t.Parallel()

	const input = `
<!DOCTYPE html>
<html>
  <head>
    <style>
      body { background-image: url('style.png'); }
    </style>
  </head>
  <body>
    <a href="link1.png">link</a>
    <img src="image.png" data="data.json" poster="poster.jpg" srcset="one.png 1x, two.png 2x"/>
    <form action="submit.php">
      <button formaction="btn.png">ok</button>
    </form>
    <div id="style-attr" style="background: url(bg\20 1.png); color: red;"></div>
    <div id="style-attr2" style="background: url('bg\20 2.png'); color: red;"></div>
    <div id="style-attr3" style="background: url(&#34;bg\20 3.png&#34;); color: red;"></div>
    <blockquote cite="cite.pdf"></blockquote>
    <div background="bgattr.jpg"></div>
    <input type="image" formaction="input.png" src="input.png"/>
  </body>
</html>
`

	// simple sanitizer that wraps every URL in X(...)
	sanitize := func(u string) string {
		return "X(" + u + ")"
	}

	urls := make(map[string]struct{})

	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	err = HTMLSanitizeAndExtractUrls(doc, urls, sanitize)
	if err != nil {
		t.Fatalf("sanitize error: %v", err)
	}

	var buf bytes.Buffer

	err = html.Render(&buf, doc)
	if err != nil {
		t.Fatal(err)
	}

	doc, err = html.Parse(&buf)
	if err != nil {
		t.Fatalf("failed to re-parse sanitized html: %v", err)
	}

	tests := []struct {
		name      string
		selector  string
		attribute string
		expected  string
	}{
		{"href", "a", "href", "X(link1.png)"},
		{"src", "img", "src", "X(image.png)"},
		{"data", "img", "data", "X(data.json)"},
		{"poster", "img", "poster", "X(poster.jpg)"},
		{"srcset", "img", "srcset", "X(one.png) 1x, X(two.png) 2x"},
		{"form action", "form", "action", "X(submit.php)"},
		{"formaction", "button", "formaction", "X(btn.png)"},
		{"style-attr", "#style-attr", "style", "background: url(X\\(bg\\ 1.png\\)); color: red;"},
		{"style-attr2", "#style-attr2", "style", "background: url('X(bg 2.png)'); color: red;"},
		{"style-attr3", "#style-attr3", "style", "background: url(\"X(bg 3.png)\"); color: red;"},
		{"cite", "blockquote", "cite", "X(cite.pdf)"},
		{"background-attr", "[background]", "background", "X(bgattr.jpg)"},
		{"input-formaction", "input", "formaction", "X(input.png)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			node := findNode(doc, tt.selector)
			if node == nil {
				t.Fatalf("node %q not found", tt.selector)
			}

			val := getAttr(node, tt.attribute)
			if val != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, val)
			}
		})
	}

	// Test style node sanitization
	styleNode := findNode(doc, "style")
	if styleNode == nil || styleNode.FirstChild == nil {
		t.Fatal("style node not found or is empty")
	}

	expectedStyleContent := "body { background-image: url('X(style.png)'); }"
	if strings.TrimSpace(styleNode.FirstChild.Data) != expectedStyleContent {
		t.Errorf("expected style content to be %q, got %q", expectedStyleContent, styleNode.FirstChild.Data)
	}

	// URL collection check
	wantUrls := []string{
		"X(link1.png)",
		"X(image.png)",
		"X(data.json)",
		"X(poster.jpg)",
		"X(one.png)",
		"X(two.png)",
		"X(submit.php)",
		"X(btn.png)",
		"X(bg 1.png)",
		"X(bg 2.png)",
		"X(bg 3.png)",
		"X(style.png)",
		"X(cite.pdf)",
		"X(bgattr.jpg)",
		"X(input.png)",
	}

	for _, u := range wantUrls {
		if _, ok := urls[u]; !ok {
			t.Errorf("url %q was not recorded by sanitizer", u)
		}
	}
}

func TestHTMLSanitizeElements(t *testing.T) {
	t.Parallel()

	const input = `
<!DOCTYPE html>
<html>
<body>
  <h1>Welcome</h1>
  <script>alert('xss')</script>
  <p>Some text</p>
  <style>body { color: red; }</style>
  <div><script>alert('nested xss')</script></div>
</body>
</html>
`

	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	HTMLSanitizeElements(doc, []string{"script", "style"})

	var buf bytes.Buffer

	err = html.Render(&buf, doc)
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()

	if strings.Contains(out, "<script>") {
		t.Error("script tag should have been removed")
	}

	if strings.Contains(out, "<style>") {
		t.Error("style tag should have been removed")
	}

	if !strings.Contains(out, "<h1>Welcome</h1>") {
		t.Error("h1 tag should be present")
	}
}

func findNode(n *html.Node, selector string) *html.Node {
	var walk func(*html.Node) *html.Node
	walk = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode {
			switch {
			case strings.HasPrefix(selector, "#"):
				if getAttr(n, "id") == selector[1:] {
					return n
				}
			case strings.HasPrefix(selector, "[") && strings.HasSuffix(selector, "]"):
				if hasAttr(n, selector[1:len(selector)-1]) {
					return n
				}
			default:
				if n.Data == selector {
					return n
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if found := walk(c); found != nil {
				return found
			}
		}

		return nil
	}

	return walk(n)
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}

	return ""
}

func hasAttr(n *html.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return true
		}
	}

	return false
}
