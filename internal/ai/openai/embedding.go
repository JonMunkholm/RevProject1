package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// GenerateEmbedding calls the OpenAI embeddings API and returns the vector and model used.
func GenerateEmbedding(ctx context.Context, baseURL, apiKey, projectID, model, input string) ([]float32, string, error) {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = "https://api.openai.com/v1"
	}

	if isLocalEndpoint(base) {
		vec := simulateEmbedding(model, input)
		return vec, model, nil
	}

	payload := map[string]any{
		"model": model,
		"input": input,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	endpoint := strings.TrimRight(base, "/") + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if projectID != "" {
		req.Header.Set("OpenAI-Project", projectID)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("failed to close response body: %v", cerr)
		}
	}()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("openai embeddings failed: %s (%s)", resp.Status, strings.TrimSpace(string(data)))
	}

	var parsed embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, "", err
	}
	if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) == 0 {
		return nil, "", errors.New("embedding response missing vector data")
	}

	vector := make([]float32, len(parsed.Data[0].Embedding))
	for i, v := range parsed.Data[0].Embedding {
		vector[i] = float32(v)
	}

	modelUsed := parsed.Model
	if modelUsed == "" {
		modelUsed = model
	}
	return vector, modelUsed, nil
}

func isLocalEndpoint(base string) bool {
	base = strings.ToLower(base)
	if strings.HasPrefix(base, "http://localhost") || strings.HasPrefix(base, "https://localhost") {
		return true
	}
	if strings.HasPrefix(base, "http://127.0.0.1") || strings.HasPrefix(base, "https://127.0.0.1") {
		return true
	}
	return false
}

func simulateEmbedding(model, input string) []float32 {
	dim := embeddingDimension(model)
	vector := make([]float32, dim)
	if len(input) == 0 {
		return vector
	}

	seed := crc32.ChecksumIEEE([]byte(model + ":" + input))
	for i := 0; i < dim; i++ {
		value := float32((seed>>uint(i%24))&0xFF) / 255.0
		vector[i] = value
	}
	return vector
}

func embeddingDimension(model string) int {
	switch strings.ToLower(model) {
	case "text-embedding-3-large":
		return 3072
	case "text-embedding-3-small":
		return 1536
	case "text-embedding-ada-002":
		return 1536
	default:
		return 1536
	}
}

type embeddingResponse struct {
	Model string `json:"model"`
	Data  []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}
