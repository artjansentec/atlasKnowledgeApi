package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/atlas/knowledge-api/pkg/httperr"
)

// ExecutionListFilter filtros encaminhados ao Mnemos GET /v1/observability/executions.
type ExecutionListFilter struct {
	Limit     int
	Operation string
	Provider  string
	ProjectID string
	Status    string
	Model     string
}

// ExecutionListResponse espelha a lista resumida do Mnemos.
type ExecutionListResponse struct {
	Items []ExecutionSummary `json:"items"`
	Count int                `json:"count"`
}

// ExecutionSummary resumo de uma execução (sem stages/documents/costs/errors).
type ExecutionSummary struct {
	ID                         string   `json:"id"`
	ProjectID                  string   `json:"project_id"`
	RequestID                  string   `json:"request_id,omitempty"`
	Provider                   string   `json:"provider"`
	Model                      string   `json:"model"`
	Operation                  string   `json:"operation"`
	Status                     string   `json:"status"`
	StartedAt                  string   `json:"started_at"`
	FinishedAt                 string   `json:"finished_at,omitempty"`
	DurationMs                 int64    `json:"duration_ms"`
	PromptTokens               int      `json:"prompt_tokens"`
	CompletionTokens           int      `json:"completion_tokens"`
	TotalTokens                int      `json:"total_tokens"`
	PromptCost                 float64  `json:"prompt_cost"`
	CompletionCost             float64  `json:"completion_cost"`
	TotalCost                  float64  `json:"total_cost"`
	Currency                   string   `json:"currency"`
	DocumentsFound             int      `json:"documents_found"`
	DocumentsUsed              int      `json:"documents_used"`
	ChunksFound                int      `json:"chunks_found"`
	ChunksUsed                 int      `json:"chunks_used"`
	ChunkExpansionSize         int      `json:"chunk_expansion_size,omitempty"`
	ContextWindowSize          int      `json:"context_window_size,omitempty"`
	EmbeddingModel             string   `json:"embedding_model,omitempty"`
	EmbeddingDurationMs        int64    `json:"embedding_duration_ms,omitempty"`
	VectorSearchDurationMs     int64    `json:"vector_search_duration_ms,omitempty"`
	HybridSearchDurationMs     int64    `json:"hybrid_search_duration_ms,omitempty"`
	RerankDurationMs           int64    `json:"rerank_duration_ms,omitempty"`
	ContextExpansionDurationMs int64    `json:"context_expansion_duration_ms,omitempty"`
	PromptBuildDurationMs      int64    `json:"prompt_build_duration_ms,omitempty"`
	LLMDurationMs              int64    `json:"llm_duration_ms,omitempty"`
	GroundingDurationMs        int64    `json:"grounding_duration_ms,omitempty"`
	PostProcessDurationMs      int64    `json:"post_process_duration_ms,omitempty"`
	TotalPipelineDurationMs    int64    `json:"total_pipeline_duration_ms,omitempty"`
	Grounded                   bool     `json:"grounded"`
	GroundingScore             float64  `json:"grounding_score"`
	ConfidenceScore            float64  `json:"confidence_score,omitempty"`
	RetrievalScore             float64  `json:"retrieval_score,omitempty"`
	RerankScore                float64  `json:"rerank_score,omitempty"`
	CacheHit                   bool     `json:"cache_hit"`
	ResponseLength             int      `json:"response_length,omitempty"`
}

// ExecutionDetail é o resumo + filhos (stages, documents, costs, errors).
type ExecutionDetail struct {
	ExecutionSummary
	Stages    []ExecutionStage    `json:"stages"`
	Documents []ExecutionDocument `json:"documents"`
	Costs     []ExecutionCost     `json:"costs"`
	Errors    []ExecutionError    `json:"errors"`
}

// ExecutionStage estágio do pipeline.
type ExecutionStage struct {
	StageName  string         `json:"stage_name"`
	StartedAt  string         `json:"started_at,omitempty"`
	FinishedAt string         `json:"finished_at,omitempty"`
	DurationMs int64          `json:"duration_ms"`
	Status     string         `json:"status"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// ExecutionDocument chunk/documento associado à execução.
type ExecutionDocument struct {
	DocumentID       string  `json:"document_id"`
	ChunkID          string  `json:"chunk_id"`
	ChunkIndex       int     `json:"chunk_index"`
	SimilarityScore  float64 `json:"similarity_score"`
	RerankScore      float64 `json:"rerank_score"`
	WasUsedInPrompt  bool    `json:"was_used_in_prompt"`
	Position         int     `json:"position"`
}

// ExecutionCost detalhe de custo por provider/model.
type ExecutionCost struct {
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	Currency         string  `json:"currency"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	PromptCost       float64 `json:"prompt_cost"`
	CompletionCost   float64 `json:"completion_cost"`
	TotalCost        float64 `json:"total_cost"`
}

// ExecutionError erro ocorrido em algum estágio.
type ExecutionError struct {
	Stage      string `json:"stage"`
	Type       string `json:"type"`
	Message    string `json:"message"`
	OccurredAt string `json:"occurred_at"`
}

// ListExecutions consulta GET {base}/v1/observability/executions.
func (c *Client) ListExecutions(ctx context.Context, filter ExecutionListFilter) (*ExecutionListResponse, error) {
	if c.baseURL == "" {
		return nil, httperr.BadGateway("serviço de IA não configurado (AI_SERVICE_URL)")
	}

	q := url.Values{}
	if filter.Limit > 0 {
		q.Set("limit", strconv.Itoa(filter.Limit))
	}
	if v := strings.TrimSpace(filter.Operation); v != "" {
		q.Set("operation", v)
	}
	if v := strings.TrimSpace(filter.Provider); v != "" {
		q.Set("provider", v)
	}
	if v := strings.TrimSpace(filter.ProjectID); v != "" {
		q.Set("project_id", v)
	}
	if v := strings.TrimSpace(filter.Status); v != "" {
		q.Set("status", v)
	}
	if v := strings.TrimSpace(filter.Model); v != "" {
		q.Set("model", v)
	}

	endpoint := c.baseURL + "/v1/observability/executions"
	if encoded := q.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	raw, status, err := c.getObservability(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	if err := mapObservabilityStatus(status, raw, "listar execuções"); err != nil {
		return nil, err
	}

	var out ExecutionListResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, httperr.BadGateway("resposta de observability do Serviço de IA inválida")
	}
	if out.Items == nil {
		out.Items = []ExecutionSummary{}
	}
	return &out, nil
}

// GetExecution consulta GET {base}/v1/observability/executions/{id}.
func (c *Client) GetExecution(ctx context.Context, id string) (*ExecutionDetail, error) {
	if c.baseURL == "" {
		return nil, httperr.BadGateway("serviço de IA não configurado (AI_SERVICE_URL)")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, httperr.BadRequest("id da execução é obrigatório")
	}

	endpoint := c.baseURL + "/v1/observability/executions/" + url.PathEscape(id)
	raw, status, err := c.getObservability(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, httperr.NotFound("execução não encontrada")
	}
	if err := mapObservabilityStatus(status, raw, "obter execução"); err != nil {
		return nil, err
	}

	var out ExecutionDetail
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, httperr.BadGateway("resposta de observability do Serviço de IA inválida")
	}
	if out.Stages == nil {
		out.Stages = []ExecutionStage{}
	}
	if out.Documents == nil {
		out.Documents = []ExecutionDocument{}
	}
	if out.Costs == nil {
		out.Costs = []ExecutionCost{}
	}
	if out.Errors == nil {
		out.Errors = []ExecutionError{}
	}
	return &out, nil
}

func (c *Client) getObservability(ctx context.Context, endpoint string) ([]byte, int, error) {
	timeout := c.timeout
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 30 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, httperr.Internal("falha ao criar requisição de observability")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, mapTransportError(reqCtx, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, 0, httperr.BadGateway("falha ao ler resposta de observability do Serviço de IA")
	}
	return raw, resp.StatusCode, nil
}

func mapObservabilityStatus(status int, raw []byte, action string) error {
	if status == http.StatusOK {
		return nil
	}
	if status == http.StatusBadRequest {
		msg := extractAIError(raw)
		if msg == "" {
			msg = "requisição de observability inválida"
		}
		return httperr.BadGateway(msg)
	}
	if status >= 500 {
		return httperr.BadGateway(fmt.Sprintf("serviço de IA retornou status %d ao %s", status, action))
	}
	if status >= 400 {
		return httperr.BadGateway(fmt.Sprintf("serviço de IA rejeitou observability (status %d)", status))
	}
	return httperr.BadGateway(fmt.Sprintf("serviço de IA retornou status %d ao %s", status, action))
}
