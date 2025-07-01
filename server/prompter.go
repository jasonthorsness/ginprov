package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/jasonthorsness/ginprov/gemini"
)

type Prompter interface {
	GetPromptForSlug(ctx context.Context, slug, links string, progress func(string)) (string, error)
}

func NewPrompter(gemini *gemini.Client, site string, root *os.Root, rootPath string) Prompter {
	return &defaultPrompter{gemini, site, root, rootPath, "", sync.Mutex{}}
}

type defaultPrompter struct {
	gemini   *gemini.Client
	site     string
	root     *os.Root
	rootPath string
	outline  string
	mu       sync.Mutex
}

var ErrUnsafe = errors.New("unsafe topic")

const unsafeOutline = "UNSAFE"

const outlineTXT = "outline.txt"

const safetyTemplate = `
If the following topic is appropriate for all ages and audiences, respond with the single word "SAFE": {{slug}}.
`

const outlineTemplate = `
You are a professional web designer. Create a concise outline in markdown format of a site for the topic "{{slug}}".

Construction rules:
- Do **not** reference any external resources (fonts, CDNs, embeds, etc.).
- No JavaScript or SVG.
- All images must be JPG.
- All <img> tags must have a width and height.
- In CSS any image use must include object-fit: cover.
- All links must be relative to the root and be a long, descriptive slug of the content (like company-owner-with-hat.jpg
  or goat-facts-continued.html).
- The site must display well on both desktop and mobile devices.
- Never use position: sticky;

The outline should include the following sections: 
- **Site Name And Paragraph Summary** - a short description of the site and its purpose
- **Style guide** – typography (built-in font classes only), spacing, imagery tone, theme
- **Color scheme** – primary, secondary, accent, neutrals (name + hex)  
- **Layout** – grid/flex description, breakpoints, reusable components  
- **Site map** – unordered list of important pages with slug filenames
- **Key features** – bullet list
- **Reusable CSS/HTML snippet** – fenced code blocks showing the skeleton for nav, hero, article, and footer

Write clearly enough that different teammates could each build a page and the finished site will remain cohesive and 
on-brand.Make sure you capture the essence of the topic in the design, be creative!
`

const htmlTemplate = `
You are a professional web designer. Your colleague has produced a site outline for you to follow, and your task is to 
produce a single HTML page {{slug}} within that site using that outline.

Construction rules:
- Do **not** reference any external resources (fonts, CDNs, embeds, etc.).
- No JavaScript or SVG.
- All images must be JPG.
- All <img> tags must have a width and height.
- In CSS any image use must include object-fit: cover.
- All links must be relative to the root and be a long, descriptive slug of the content (like company-owner-with-hat.jpg
  or goat-facts-continued.html).
- The site must display well on both desktop and mobile devices.
- Never use position: sticky;

The page YOU are producing is {{slug}}.

Here is the outline to help guide you in your design:

{{outline}}

Here are some other pages (non-exhaustive list) or images on the site you might consider using or linking to:

{{links}}
`

const imageTemplate = `
Create an image to be used on the web site {{site}}. The image you are creating is called {{slug}}.
`

func (p *defaultPrompter) GetPromptForSlug(
	ctx context.Context,
	slug string,
	links string,
	progress func(string),
) (string, error) {
	p.mu.Lock()
	outline := p.outline
	p.mu.Unlock()

	if outline == "" {
		err := p.initOutline(ctx, progress)
		if err != nil {
			return "", err
		}

		p.mu.Lock()
		outline = p.outline
		p.mu.Unlock()
	}

	if outline == unsafeOutline {
		return "", ErrUnsafe
	}

	if strings.HasSuffix(slug, ".jpg") {
		prompt := strings.ReplaceAll(imageTemplate, "{{slug}}", slug)
		prompt = strings.ReplaceAll(prompt, "{{site}}", p.site)

		return prompt, nil
	}

	prompt := strings.ReplaceAll(htmlTemplate, "{{slug}}", slug)
	prompt = strings.ReplaceAll(prompt, "{{outline}}", outline)
	prompt = strings.ReplaceAll(prompt, "{{links}}", links)

	return prompt, nil
}

func (p *defaultPrompter) initOutline(ctx context.Context, progress func(string)) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.outline != "" {
		return nil
	}

	f, err := p.root.Open(outlineTXT)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read file: %s: %w", outlineTXT, err)
		}

		err = p.genOutline(ctx, progress)
		if err != nil {
			return err
		}

		err = writeFileAtomic(p.root, p.rootPath, outlineTXT, []byte(p.outline))
		if err != nil {
			return err
		}

		return nil
	}

	v, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("failed to read prompt file %s: %w", p.site, err)
	}

	p.outline = string(v)

	return nil
}

func (p *defaultPrompter) genOutline(ctx context.Context, progress func(string)) error {
	safetyPrompt := strings.ReplaceAll(safetyTemplate, "{{slug}}", p.site)

	safe, err := p.gemini.Text(ctx, safetyPrompt, progress)
	if err != nil {
		return fmt.Errorf("failed to get safety assessment from gemini: %w", err)
	}

	if safe != "SAFE" {
		p.outline = unsafeOutline
		return nil
	}

	progress("\nGenerating outline...\n")

	outlinePrompt := strings.ReplaceAll(outlineTemplate, "{{slug}}", p.site)

	outline, err := p.gemini.Text(ctx, outlinePrompt, progress)
	if err != nil {
		return fmt.Errorf("failed to get outline from gemini: %w", err)
	}

	p.outline = outline

	progress("\n")

	return nil
}
