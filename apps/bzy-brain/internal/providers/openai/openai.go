// Package openai implements the Provider interface using the OpenAI API.
// Compatible with any OpenAI-API-compatible endpoint (Azure, Together, Groq, local LiteLLM).
package openai

import (
	"context"
	"fmt"
	"io"

	goopenai "github.com/sashabaranov/go-openai"

	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/providers"
)

type Provider struct {
	client     *goopenai.Client
	chatModel  string
	embedModel string
	dims       int
}

func New(apiKey, baseURL, chatModel, embedModel string) *Provider {
	cfg := goopenai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &Provider{
		client:     goopenai.NewClientWithConfig(cfg),
		chatModel:  chatModel,
		embedModel: embedModel,
		dims:       1536,
	}
}

func (p *Provider) ID() string { return "openai" }

func (p *Provider) ModelID() string { return p.chatModel }

func (p *Provider) Dimensions() int { return p.dims }

func (p *Provider) HealthCheck(ctx context.Context) error {
	_, err := p.client.ListModels(ctx)
	return err
}

func (p *Provider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	msgs := convertMessages(req.Messages)
	tools := convertTools(req.Tools)

	resp, err := p.client.CreateChatCompletion(ctx, goopenai.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    msgs,
		Tools:       tools,
		MaxTokens:   req.MaxTokens,
		Temperature: float32(req.Temperature),
	})
	if err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}

	choice := resp.Choices[0]
	out := &providers.ChatResponse{
		ID:    resp.ID,
		Model: resp.Model,
		Message: providers.Message{
			Role:    providers.Role(choice.Message.Role),
			Content: choice.Message.Content,
		},
		Usage: providers.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		Finish: string(choice.FinishReason),
	}
	for _, tc := range choice.Message.ToolCalls {
		out.Message.ToolCalls = append(out.Message.ToolCalls, providers.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	return out, nil
}

func (p *Provider) ChatStream(ctx context.Context, req providers.ChatRequest) (providers.StreamReader, error) {
	msgs := convertMessages(req.Messages)
	stream, err := p.client.CreateChatCompletionStream(ctx, goopenai.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    msgs,
		MaxTokens:   req.MaxTokens,
		Temperature: float32(req.Temperature),
		Stream:      true,
	})
	if err != nil {
		return nil, fmt.Errorf("openai stream: %w", err)
	}
	return &streamReader{stream: stream}, nil
}

func (p *Provider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := p.client.CreateEmbeddings(ctx, goopenai.EmbeddingRequest{
		Input: texts,
		Model: goopenai.EmbeddingModel(p.embedModel),
	})
	if err != nil {
		return nil, fmt.Errorf("openai embed: %w", err)
	}
	out := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		out[i] = d.Embedding
	}
	return out, nil
}

// ─── stream adapter ────────────────────────────────────────────────────────────

type streamReader struct {
	stream *goopenai.ChatCompletionStream
}

func (s *streamReader) Next() (*providers.StreamChunk, error) {
	resp, err := s.stream.Recv()
	if err == io.EOF {
		return &providers.StreamChunk{Done: true}, nil
	}
	if err != nil {
		return nil, err
	}
	delta := ""
	if len(resp.Choices) > 0 {
		delta = resp.Choices[0].Delta.Content
	}
	return &providers.StreamChunk{Delta: delta}, nil
}

func (s *streamReader) Close() error {
	s.stream.Close()
	return nil
}

// ─── converters ────────────────────────────────────────────────────────────────

func convertMessages(msgs []providers.Message) []goopenai.ChatCompletionMessage {
	out := make([]goopenai.ChatCompletionMessage, len(msgs))
	for i, m := range msgs {
		out[i] = goopenai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}
	return out
}

func convertTools(specs []providers.ToolSpec) []goopenai.Tool {
	out := make([]goopenai.Tool, len(specs))
	for i, s := range specs {
		out[i] = goopenai.Tool{
			Type: goopenai.ToolTypeFunction,
			Function: &goopenai.FunctionDefinition{
				Name:        s.Name,
				Description: s.Description,
				Parameters:  s.Parameters,
			},
		}
	}
	return out
}
