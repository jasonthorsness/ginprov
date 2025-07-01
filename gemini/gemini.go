package gemini

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"google.golang.org/genai"
)

const (
	htmlModel  = "gemini-2.5-flash-lite-preview-06-17"
	imageModel = "gemini-2.0-flash-preview-image-generation"
)

const (
	htmlSystemInstructions = "Return only HTML"
)

type Client struct {
	client *genai.Client
}

var ErrResponseUnexpected = errors.New("unexpected response from Gemini")

func New(ctx context.Context, apiKey string) (*Client, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize gemini client: %w", err)
	}

	return &Client{client}, nil
}

func (g *Client) Close() error {
	return nil
}

func (g *Client) HTML(ctx context.Context, prompt string, progress func(string)) (*html.Node, error) {
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: htmlSystemInstructions},
			},
			Role: "",
		},
	}

	var sb strings.Builder

	stream := g.client.Models.GenerateContentStream(ctx, htmlModel, genai.Text(prompt), config)
	for chunk, err := range stream {
		if err != nil {
			return nil, err
		}

		parts, err := extractSingleCandidateParts(chunk)
		if err != nil {
			return nil, err
		}

		if len(parts) == 0 {
			continue
		}

		if len(parts) != 1 {
			return nil, fmt.Errorf("%w: expected one part, got %d", ErrResponseUnexpected, len(parts))
		}

		if progress != nil {
			progress(parts[0].Text)
		}

		sb.WriteString(parts[0].Text)
	}

	raw := sb.String()

	start := strings.Index(raw, "<html")
	if start < 0 {
		return nil, fmt.Errorf("%w: no <html> tag found in response", ErrResponseUnexpected)
	}

	raw = raw[start:]

	end := strings.LastIndex(raw, "</html>")
	if end < 0 {
		return nil, fmt.Errorf("%w: no </html> closing tag found in response", ErrResponseUnexpected)
	}

	raw = raw[:end+len("</html>")]

	result, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("html.Parse failed: %w", err)
	}

	return result, nil
}

func (g *Client) PNG(ctx context.Context, prompt string, progress func(string)) ([]byte, error) {
	config := &genai.GenerateContentConfig{
		ResponseModalities: []string{"TEXT", "IMAGE"},
	}

	stream := g.client.Models.GenerateContentStream(
		ctx,
		imageModel,
		genai.Text(prompt),
		config,
	)

	var imageBytes []byte

	for chunk, err := range stream {
		if err != nil {
			return nil, fmt.Errorf("gemini error %w", err)
		}

		parts, err := extractSingleCandidateParts(chunk)
		if err != nil {
			return nil, err
		}

		for _, part := range parts {
			if progress != nil && part.Text != "" {
				progress(part.Text)
			}

			if part.InlineData != nil {
				if len(imageBytes) > 0 {
					return nil, fmt.Errorf("%w: multiple image parts received", ErrResponseUnexpected)
				}

				imageBytes = part.InlineData.Data
			}
		}
	}

	if len(imageBytes) == 0 {
		return nil, fmt.Errorf("%w: no image received", ErrResponseUnexpected)
	}

	return imageBytes, nil
}

func (g *Client) Text(ctx context.Context, prompt string, progress func(string)) (string, error) {
	config := &genai.GenerateContentConfig{}

	var sb strings.Builder

	stream := g.client.Models.GenerateContentStream(ctx, htmlModel, genai.Text(prompt), config)
	for chunk, err := range stream {
		if err != nil {
			return "", err
		}

		parts, err := extractSingleCandidateParts(chunk)
		if err != nil {
			return "", err
		}

		if len(parts) == 0 {
			continue
		}

		if len(parts) != 1 {
			return "", fmt.Errorf("%w: expected one part, got %d", ErrResponseUnexpected, len(parts))
		}

		if progress != nil {
			progress(parts[0].Text)
		}

		sb.WriteString(parts[0].Text)
	}

	result := sb.String()

	return result, nil
}

func extractSingleCandidateParts(v *genai.GenerateContentResponse) ([]*genai.Part, error) {
	if len(v.Candidates) != 1 {
		return nil, fmt.Errorf("%w: expected one candidate, got %d", ErrResponseUnexpected, len(v.Candidates))
	}

	if v.Candidates[0].Content == nil {
		return nil, nil // no parts
	}

	return v.Candidates[0].Content.Parts, nil
}
