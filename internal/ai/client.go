package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/atlas/knowledge-api/pkg/httperr"
)

// GenerateRequest agrupa os dados enviados ao Serviço de IA.
type GenerateRequest struct {
	ProjectID         string
	ProjectSlug       string
	ProjectName       string
	Description       string
	GenerationOptions json.RawMessage
	Files             []FileInput
	// RequestedBy é o UUID do usuário logado no Atlas (responsável no sync de volta).
	RequestedBy string
	// OnProgress é chamado a cada atualização do job remoto (opcional).
	OnProgress func(JobStatus)
}

// FileInput é um arquivo a ser enviado no multipart.
type FileInput struct {
	Name     string
	MimeType string
	Reader   io.Reader
}

// GenerateResponse é o resultado final do Serviço de IA.
type GenerateResponse struct {
	Content   json.RawMessage `json:"content"`
	ModelUsed string          `json:"model"`
	Language  string          `json:"language"`
	Raw       json.RawMessage `json:"-"`
}

// JobStatus espelha o estado retornado por GET /v1/jobs/:id.
type JobStatus struct {
	ID           string          `json:"id"`
	Status       string          `json:"status"`
	Progress     int             `json:"progress"`
	CurrentStage string          `json:"current_stage"`
	Errors       []string        `json:"errors,omitempty"`
	Result       json.RawMessage `json:"result,omitempty"`
	DurationMs   int64           `json:"duration_ms,omitempty"`
}

type createDocumentResponse struct {
	JobID     string `json:"job_id"`
	Status    string `json:"status"`
	FileCount int    `json:"file_count"`
}

// Client comunica-se exclusivamente com o Serviço de IA (Mnemos).
type Client struct {
	baseURL    string
	timeout    time.Duration
	httpClient *http.Client
	pollEvery  time.Duration
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		timeout: timeout,
		httpClient: &http.Client{
			// Timeout por requisição via context; o cliente não corta o polling.
			Timeout: 0,
		},
		pollEvery: 2 * time.Second,
	}
}

// Generate envia arquivos ao Mnemos, acompanha o job e devolve o resultado.
func (c *Client) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	jobID, err := c.submitDocument(ctx, req)
	if err != nil {
		return nil, err
	}

	job, err := c.waitJob(ctx, jobID, req.OnProgress)
	if err != nil {
		return nil, err
	}

	if strings.EqualFold(job.Status, "FAILED") {
		msg := "serviço de IA falhou no processamento"
		if len(job.Errors) > 0 {
			msg = strings.Join(job.Errors, "; ")
		}
		return nil, httperr.BadGateway(msg)
	}

	if len(job.Result) == 0 {
		return nil, httperr.BadGateway("serviço de IA concluiu sem resultado")
	}

	modelUsed := ""
	var meta struct {
		Metadata struct {
			Model string `json:"model"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(job.Result, &meta); err == nil {
		modelUsed = meta.Metadata.Model
	}

	raw, _ := json.Marshal(job)
	return &GenerateResponse{
		Content:   job.Result,
		ModelUsed: modelUsed,
		Language:  "",
		Raw:       raw,
	}, nil
}

func (c *Client) submitDocument(ctx context.Context, req GenerateRequest) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Identity hints — Mnemos must reuse these on Atlas Sync to avoid duplicate projects.
	if req.ProjectID != "" {
		_ = writer.WriteField("project_id", req.ProjectID)
	}
	if req.ProjectSlug != "" {
		_ = writer.WriteField("project_slug", req.ProjectSlug)
		_ = writer.WriteField("slug", req.ProjectSlug)
	}
	if req.ProjectName != "" {
		_ = writer.WriteField("project_name", req.ProjectName)
	}
	if req.Description != "" {
		_ = writer.WriteField("description", req.Description)
	}
	if len(req.GenerationOptions) > 0 {
		_ = writer.WriteField("generation_options", string(req.GenerationOptions))
	}
	if req.RequestedBy != "" {
		_ = writer.WriteField("responsible_user_id", req.RequestedBy)
		_ = writer.WriteField("requested_by", req.RequestedBy)
	}

	for _, f := range req.Files {
		part, err := writer.CreateFormFile("files", filepath.Base(f.Name))
		if err != nil {
			return "", httperr.Internal("falha ao montar multipart para o Serviço de IA")
		}
		if _, err := io.Copy(part, f.Reader); err != nil {
			return "", httperr.Internal("falha ao anexar arquivo para o Serviço de IA")
		}
	}

	if err := writer.Close(); err != nil {
		return "", httperr.Internal("falha ao finalizar multipart para o Serviço de IA")
	}

	uploadCtx, cancel := context.WithTimeout(ctx, uploadTimeout(c.timeout))
	defer cancel()

	url := c.baseURL + "/v1/knowledge/document"
	httpReq, err := http.NewRequestWithContext(uploadCtx, http.MethodPost, url, &body)
	if err != nil {
		return "", httperr.Internal("falha ao criar requisição para o Serviço de IA")
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", mapTransportError(ctx, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", httperr.BadGateway("falha ao ler resposta do Serviço de IA")
	}

	if resp.StatusCode >= 500 {
		return "", httperr.BadGateway(fmt.Sprintf("serviço de IA retornou status %d", resp.StatusCode))
	}
	if resp.StatusCode >= 400 {
		return "", httperr.BadGateway(fmt.Sprintf("serviço de IA rejeitou a requisição (status %d)", resp.StatusCode))
	}
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return "", httperr.BadGateway(fmt.Sprintf("serviço de IA retornou status inesperado %d", resp.StatusCode))
	}

	var created createDocumentResponse
	if err := json.Unmarshal(raw, &created); err != nil || created.JobID == "" {
		return "", httperr.BadGateway("serviço de IA não retornou job_id")
	}
	return created.JobID, nil
}

func (c *Client) waitJob(ctx context.Context, jobID string, onProgress func(JobStatus)) (*JobStatus, error) {
	ticker := time.NewTicker(c.pollEvery)
	defer ticker.Stop()

	for {
		job, err := c.getJob(ctx, jobID)
		if err != nil {
			return nil, err
		}
		if onProgress != nil {
			onProgress(*job)
		}

		switch strings.ToUpper(job.Status) {
		case "COMPLETED", "FAILED":
			return job, nil
		}

		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return nil, httperr.GatewayTimeout("timeout ao aguardar resultado do Serviço de IA")
			}
			return nil, httperr.BadGateway("comunicação com o Serviço de IA cancelada")
		case <-ticker.C:
		}
	}
}

func (c *Client) getJob(ctx context.Context, jobID string) (*JobStatus, error) {
	pollCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	url := c.baseURL + "/v1/jobs/" + jobID
	httpReq, err := http.NewRequestWithContext(pollCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, httperr.Internal("falha ao criar consulta de job no Serviço de IA")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, mapTransportError(ctx, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, httperr.BadGateway("falha ao ler status do job no Serviço de IA")
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, httperr.BadGateway("job do Serviço de IA não encontrado")
	}
	if resp.StatusCode >= 400 {
		return nil, httperr.BadGateway(fmt.Sprintf("serviço de IA retornou status %d ao consultar job", resp.StatusCode))
	}

	var job JobStatus
	if err := json.Unmarshal(raw, &job); err != nil {
		return nil, httperr.BadGateway("resposta de job do Serviço de IA inválida")
	}
	return &job, nil
}

func uploadTimeout(overall time.Duration) time.Duration {
	if overall <= 0 {
		return 2 * time.Minute
	}
	if overall < 2*time.Minute {
		return overall
	}
	return 2 * time.Minute
}

func mapTransportError(ctx context.Context, err error) error {
	if ctx.Err() == context.DeadlineExceeded || isTimeout(err) {
		return httperr.GatewayTimeout("timeout ao comunicar com o Serviço de IA")
	}
	return httperr.BadGateway("serviço de IA indisponível")
}

func isTimeout(err error) bool {
	type timeout interface{ Timeout() bool }
	if t, ok := err.(timeout); ok {
		return t.Timeout()
	}
	return false
}
