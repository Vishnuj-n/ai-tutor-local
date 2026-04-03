package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"ai-tutor-local/internal/db"
)

// SyncPayload represents the batched sync request sent to the cloud.
type SyncPayload struct {
	StudentID string  `json:"student_id"`
	ClassID   string  `json:"class_id,omitempty"`
	SyncedAt  string  `json:"synced_at"`
	Events    []Event `json:"events"`
}

// SyncResponse represents the cloud response to a sync request.
type SyncResponse struct {
	Success        bool   `json:"success"`
	EventsAccepted int    `json:"events_accepted"`
	EventsRejected int    `json:"events_rejected"`
	Message        string `json:"message,omitempty"`
}

// TransportClient handles HTTP communication with the cloud sync endpoint.
type TransportClient struct {
	baseURL    string
	httpClient *http.Client
	queries    *db.SyncQueueQueries
}

// NewTransportClient creates a new sync transport client.
func NewTransportClient(baseURL string, queries *db.SyncQueueQueries) *TransportClient {
	return &TransportClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		queries: queries,
	}
}

// SendBatch transmits a batch of events to the cloud sync endpoint.
// Returns the number of events successfully synced and any error encountered.
func (t *TransportClient) SendBatch(ctx context.Context, studentID, classID string, items []db.SyncQueueItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	events := make([]Event, 0, len(items))
	for _, item := range items {
		var event Event
		if err := json.Unmarshal([]byte(item.Payload), &event); err != nil {
			// Mark individual item as failed and continue
			if markErr := t.queries.MarkAttempt(item.ID, "failed"); markErr != nil {
				return 0, fmt.Errorf("mark invalid payload failed for %s: %w", item.ID, markErr)
			}
			continue
		}
		events = append(events, event)
	}

	if len(events) == 0 {
		return 0, nil
	}

	payload := SyncPayload{
		StudentID: studentID,
		ClassID:   classID,
		SyncedAt:  time.Now().UTC().Format(time.RFC3339),
		Events:    events,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal sync payload: %w", err)
	}

	syncURL := t.baseURL + "/api/v1/sync"
	req, err := http.NewRequestWithContext(ctx, "POST", syncURL, bytes.NewReader(jsonData))
	if err != nil {
		return 0, fmt.Errorf("create sync request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Student-Token", studentID)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		// Mark all items as failed on network error
		for _, item := range items {
			if markErr := t.queries.MarkAttempt(item.ID, "failed"); markErr != nil {
				return 0, fmt.Errorf("mark network error failed for %s: %w", item.ID, markErr)
			}
		}
		return 0, fmt.Errorf("POST %s: %w", syncURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read response body: %w", err)
	}

	// Handle success responses (200 OK and 409 Conflict for duplicates)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusConflict {
		// Mark all items as successfully sent
		successCount := 0
		for _, item := range items {
			if markErr := t.queries.MarkAttempt(item.ID, "sent"); markErr != nil {
				return successCount, fmt.Errorf("mark sent failed for %s: %w", item.ID, markErr)
			}
			successCount++
		}
		return successCount, nil
	}

	// Handle other error responses
	var syncResp SyncResponse
	if err := json.Unmarshal(body, &syncResp); err != nil {
		// Failed to parse error response, mark all as failed
		for _, item := range items {
			if markErr := t.queries.MarkAttempt(item.ID, "failed"); markErr != nil {
				return 0, fmt.Errorf("mark failed for %s: %w", item.ID, markErr)
			}
		}
		return 0, fmt.Errorf("sync failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Mark all as failed for non-success status
	for _, item := range items {
		if markErr := t.queries.MarkAttempt(item.ID, "failed"); markErr != nil {
			return 0, fmt.Errorf("mark failed for %s: %w", item.ID, markErr)
		}
	}

	return 0, fmt.Errorf("sync failed: %s", syncResp.Message)
}

// RunSync processes pending items from the sync queue and transmits them to the cloud.
func (t *TransportClient) RunSync(ctx context.Context, studentID, classID string, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = 50
	}

	items, err := t.queries.ListRetryable(batchSize)
	if err != nil {
		return 0, fmt.Errorf("list retryable items: %w", err)
	}

	if len(items) == 0 {
		return 0, nil
	}

	// Filter items that are ready for retry (respecting backoff)
	now := time.Now().UTC()
	readyItems := make([]db.SyncQueueItem, 0, len(items))
	for _, item := range items {
		delay := retryDelay(item, now)
		if delay <= 0 {
			readyItems = append(readyItems, item)
		}
	}

	if len(readyItems) == 0 {
		return 0, nil
	}

	return t.SendBatch(ctx, studentID, classID, readyItems)
}
