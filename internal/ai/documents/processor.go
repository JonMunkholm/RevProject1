package documents

import (
	"context"
	"fmt"
	"strings"

	clientpkg "github.com/JonMunkholm/RevProject1/internal/ai/client"
)

// CredentialResolver resolves stored provider credentials.
type CredentialResolver interface {
	Resolve(ctx context.Context, reference string) (string, error)
}

// AIProcessor uses the shared AI client to execute document jobs.
type AIProcessor struct {
	client          *clientpkg.Client
	resolver        CredentialResolver
	defaultAPIKey   string
	defaultProvider string
}

// NewAIProcessor constructs a processor that delegates to the AI client.
func NewAIProcessor(client *clientpkg.Client, resolver CredentialResolver, defaultAPIKey, defaultProvider string) *AIProcessor {
	return &AIProcessor{
		client:          client,
		resolver:        resolver,
		defaultAPIKey:   defaultAPIKey,
		defaultProvider: defaultProvider,
	}
}

func (p *AIProcessor) Process(ctx context.Context, job Job) (map[string]any, error) {
	if p == nil || p.client == nil {
		return nil, fmt.Errorf("documents: ai client not configured")
	}

	providerID := job.ProviderID
	if providerID == "" {
		providerID = p.defaultProvider
	}

	opts := clientpkg.UserOptions{Provider: providerID}
	if p.defaultAPIKey != "" {
		opts.APIKey = p.defaultAPIKey
	}
	if opts.APIKey == "" && p.resolver != nil {
		reference := fmt.Sprintf("%s:%s:%s", job.CompanyID, job.UserID, providerID)
		if key, err := p.resolver.Resolve(ctx, reference); err == nil && key != "" {
			opts.APIKey = key
		}
	}
	if opts.APIKey == "" {
		return nil, fmt.Errorf("documents: no api key configured for provider %s", providerID)
	}

	prompt := buildDocumentPrompt(job)
	metadata := map[string]any{}
	if addendum, ok := job.Request["instructions"].(string); ok && addendum != "" {
		metadata = withSystemAddendum(metadata, addendum)
	}

	resp, err := p.client.Completion(ctx, opts, clientpkg.CompletionRequest{
		Prompt:   prompt,
		Metadata: metadata,
	})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"summary": resp.Text,
	}, nil
}

func buildDocumentPrompt(job Job) string {
	docs := extractDocuments(job.Request)
	instructions, _ := job.Request["instructions"].(string)

	builder := strings.Builder{}
	builder.WriteString("You are an expert revenue-recognition assistant.\n")
	builder.WriteString("Analyze the following documents and provide key findings, risks, and recommended next steps.\n\n")
	if len(docs) > 0 {
		builder.WriteString("Documents:\n")
		for i, doc := range docs {
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, doc))
		}
		builder.WriteString("\n")
	}
	if instructions != "" {
		builder.WriteString("Additional instructions: ")
		builder.WriteString(instructions)
		builder.WriteString("\n\n")
	}
	builder.WriteString("Provide a concise summary, highlight revenue recognition considerations, and suggest next steps.")
	return builder.String()
}

func extractDocuments(request map[string]any) []string {
	if request == nil {
		return nil
	}

	var docs []string
	switch v := request["documents"].(type) {
	case []string:
		docs = append(docs, v...)
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				docs = append(docs, s)
			}
		}
	}

	return docs
}

func withSystemAddendum(metadata map[string]any, addendum string) map[string]any {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	if addendum == "" {
		return metadata
	}
	switch existing := metadata["system_addendum"].(type) {
	case string:
		metadata["system_addendum"] = []string{existing, addendum}
	case []string:
		metadata["system_addendum"] = append(existing, addendum)
	case []any:
		metadata["system_addendum"] = append(existing, addendum)
	default:
		metadata["system_addendum"] = addendum
	}
	return metadata
}
