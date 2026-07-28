package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// SyncNotifier notifies Mnemos that a project's knowledge changed (fire-and-forget).
type SyncNotifier struct {
	client *Client
}

func NewSyncNotifier(client *Client) *SyncNotifier {
	return &SyncNotifier{client: client}
}

// NotifyProjectChanged triggers Mnemos full reindex for the project.
// Safe to call from request handlers; runs asynchronously.
func (n *SyncNotifier) NotifyProjectChanged(projectID string) {
	if n == nil || n.client == nil || projectID == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := n.client.NotifyProjectSync(ctx, projectID); err != nil {
			log.Printf("mnemos sync notify project=%s: %v", projectID, err)
		}
	}()
}

// NotifyProjectSync POST {base}/internal/sync/project {"project_id":"..."}.
func (c *Client) NotifyProjectSync(ctx context.Context, projectID string) error {
	if c.baseURL == "" {
		return fmt.Errorf("AI_SERVICE_URL vazio")
	}
	payload, _ := json.Marshal(map[string]string{"project_id": projectID})
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	url := c.baseURL + "/internal/sync/project"
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("mnemos sync status %d", resp.StatusCode)
	}
	return nil
}
