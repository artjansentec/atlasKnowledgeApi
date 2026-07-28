package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/atlas/knowledge-api/pkg/httperr"
)

// RAGSearchRequest é o payload enviado ao Mnemos POST /v1/rag/search.
type RAGSearchRequest struct {
	Question   string   `json:"question"`
	ProjectIDs []string `json:"project_ids"`
}

// RAGSearchResponse espelha a resposta do Mnemos.
type RAGSearchResponse struct {
	Answer     string      `json:"answer"`
	Sources    []RAGSource `json:"sources"`
	ChunksUsed int         `json:"chunks_used"`
	Score      float64     `json:"score"`
}

// RAGSource descreve um trecho usado na resposta.
type RAGSource struct {
	ProjectID   string  `json:"project_id"`
	ProjectName string  `json:"project_name"`
	Kind        string  `json:"kind"`
	SourceType  string  `json:"source_type"`
	SourceID    string  `json:"source_id"`
	Title       string  `json:"title"`
	Score       float64 `json:"score"`
}

// SearchRAG consulta o Mnemos para resposta semântica com fontes.
// projectIDs deve conter apenas projetos já autorizados pelo Atlas.
func (c *Client) SearchRAG(ctx context.Context, question string, projectIDs []string) (*RAGSearchResponse, error) {
	if c.baseURL == "" {
		return nil, httperr.BadGateway("serviço de IA não configurado (AI_SERVICE_URL)")
	}
	payload, err := json.Marshal(RAGSearchRequest{
		Question:   question,
		ProjectIDs: projectIDs,
	})
	if err != nil {
		return nil, httperr.Internal("falha ao montar requisição RAG")
	}

	timeout := c.timeout
	if timeout <= 0 || timeout > 2*time.Minute {
		timeout = 2 * time.Minute
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	url := c.baseURL + "/v1/rag/search"
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, httperr.Internal("falha ao criar requisição RAG")
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, mapTransportError(reqCtx, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, httperr.BadGateway("falha ao ler resposta RAG do Serviço de IA")
	}

	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, httperr.BadGateway("RAG desabilitado no Serviço de IA (PGVECTOR_ENABLED)")
	}
	if resp.StatusCode == http.StatusBadRequest {
		msg := extractAIError(raw)
		if msg == "" {
			msg = "requisição RAG inválida"
		}
		return nil, httperr.BadGateway(msg)
	}
	if resp.StatusCode >= 500 {
		return nil, httperr.BadGateway(fmt.Sprintf("serviço de IA retornou status %d na busca RAG", resp.StatusCode))
	}
	if resp.StatusCode >= 400 {
		return nil, httperr.BadGateway(fmt.Sprintf("serviço de IA rejeitou a busca RAG (status %d)", resp.StatusCode))
	}

	var out RAGSearchResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, httperr.BadGateway("resposta RAG do Serviço de IA inválida")
	}
	if out.Sources == nil {
		out.Sources = []RAGSource{}
	}
	return &out, nil
}

func extractAIError(raw []byte) string {
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	return body.Error
}
