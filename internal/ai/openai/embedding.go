package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GenerateEmbedding calls the OpenAI embeddings API and returns the vector and model used.
func GenerateEmbedding(ctx context.Context, baseURL, apiKey, projectID, model, input string) ([]float32, string, error) {
	payload := map[string]any{
		"model": model,
		"input": input,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/embeddings"
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
	defer resp.Body.Close()

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

type embeddingResponse struct {
	Model string `json:"model"`
	Data  []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}
