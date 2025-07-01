package sanitize

import (
	"errors"
	"strings"

	"golang.org/x/net/html"
)

const maxDepth = 100

var ErrMaxDepthExceeded = errors.New("maximum depth exceeded")

func HTMLSanitizeElements(doc *html.Node, elementsToRemove []string) {
	var nodesToRemove []*html.Node
	var walk func(*html.Node, int)
	walk = func(n *html.Node, depth int) {
		if depth > maxDepth {
			return
		}

		if n.Type == html.ElementNode {
			for _, elem := range elementsToRemove {
				if strings.EqualFold(n.Data, elem) {
					nodesToRemove = append(nodesToRemove, n)
					return
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, depth+1)
		}
	}
	walk(doc, 0)

	for _, n := range nodesToRemove {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
	}
}

//nolint:cyclop
func HTMLSanitizeAndExtractUrls(doc *html.Node, urls map[string]struct{}, sanitizeURL func(string) string) error {
	var walk func(*html.Node, int) error
	walk = func(n *html.Node, depth int) error {
		if depth > maxDepth {
			return ErrMaxDepthExceeded
		}

		if n.Type == html.ElementNode {
			for i := range n.Attr {
				switch strings.ToLower(n.Attr[i].Key) {
				case "style":
					v, err := CSSSanitizeAndExtractUrls(n.Attr[i].Val, urls, sanitizeURL)
					if err != nil {
						return err
					}

					n.Attr[i].Val = v
				case "src", "href", "action", "data", "poster", "formaction", "cite", "background", "ping", "longdesc",
					"icon", "srcdoc", "xlink:href", "codebase", "classid", "archive", "usemap", "profile", "manifest":
					v := sanitizeURL(n.Attr[i].Val)
					urls[v] = struct{}{}
					n.Attr[i].Val = v
				case "srcset", "imagesrcset":
					n.Attr[i].Val = sanitizeSrcset(n.Attr[i].Val, urls, sanitizeURL)
				}
			}

			if n.Data == "style" {
				err := sanitizeStyleNode(n, urls, sanitizeURL)
				if err != nil {
					return err
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			err := walk(c, depth+1)
			if err != nil {
				return err
			}
		}

		return nil
	}

	return walk(doc, 0)
}

func sanitizeSrcset(v string, urls map[string]struct{}, sanitizeURL func(string) string) string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}

		url := fields[0]
		desc := ""

		if len(fields) > 1 {
			desc = " " + strings.Join(fields[1:], " ")
		}

		vv := sanitizeURL(url)
		urls[vv] = struct{}{}

		out = append(out, vv+desc)
	}

	return strings.Join(out, ", ")
}

func sanitizeStyleNode(n *html.Node, urls map[string]struct{}, sanitizeURL func(string) string) error {
	var firstTextNode *html.Node
	var otherNodesExist bool
	var buf strings.Builder

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			if firstTextNode == nil {
				firstTextNode = c
			}

			buf.WriteString(c.Data)
		} else {
			otherNodesExist = true
		}
	}

	if buf.Len() == 0 {
		return nil
	}

	v, err := CSSSanitizeAndExtractUrls(buf.String(), urls, sanitizeURL)
	if err != nil {
		return err
	}

	if firstTextNode != nil && firstTextNode.NextSibling == nil && !otherNodesExist {
		firstTextNode.Data = v
		return nil
	}

	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		n.RemoveChild(c)
		c = next
	}

	n.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: v,
	})

	return nil
}
