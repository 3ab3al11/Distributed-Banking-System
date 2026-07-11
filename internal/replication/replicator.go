package replication

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"gin-sqlserver-api/internal/models"
)

const (
	OperationInsert = "insert"
	OperationUpdate = "update"
	OperationDelete = "delete"
)

type Payload struct {
	Operation string        `json:"operation"`
	Client    models.Client `json:"client"`
}

type Replicator struct {
	client    *http.Client
	slaveURLs []string
	logger    *log.Logger
	timeout   time.Duration
}

func New(slaveURLs []string, logger *log.Logger) *Replicator {
	if logger == nil {
		logger = log.Default()
	}

	return &Replicator{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		slaveURLs: slaveURLs,
		logger:    logger,
		timeout:   5 * time.Second,
	}
}

func (r *Replicator) Replicate(operation string, client models.Client) {
	payload := Payload{Operation: operation, Client: client}

	for _, slaveURL := range r.slaveURLs {
		slaveURL := slaveURL
		// Each slave is called in its own goroutine, so one offline slave cannot block the API.
		go r.send(slaveURL, payload)
	}
}

func (r *Replicator) send(slaveURL string, payload Payload) {
	body, err := json.Marshal(payload)
	if err != nil {
		r.logger.Printf("replication marshal failed for operation=%s client_id=%d: %v", payload.Operation, payload.Client.ID, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, slaveURL, bytes.NewReader(body))
	if err != nil {
		r.logger.Printf("replication request build failed slave=%s operation=%s client_id=%d: %v", slaveURL, payload.Operation, payload.Client.ID, err)
		return
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := r.client.Do(request)
	if err != nil {
		r.logger.Printf("replication failed slave=%s operation=%s client_id=%d: %v", slaveURL, payload.Operation, payload.Client.ID, err)
		return
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		r.logger.Printf("replication returned non-success slave=%s operation=%s client_id=%d status=%d", slaveURL, payload.Operation, payload.Client.ID, response.StatusCode)
		return
	}

	r.logger.Printf("replication succeeded slave=%s operation=%s client_id=%d", slaveURL, payload.Operation, payload.Client.ID)
}
