package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"gin-sqlserver-api/internal/failover"
	"gin-sqlserver-api/internal/htmlview"
	"gin-sqlserver-api/internal/models"
	"gin-sqlserver-api/internal/replication"
	slaverepositories "gin-sqlserver-api/internal/slave/repositories"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const (
	operationInsert = "insert"
	operationUpdate = "update"
	operationDelete = "delete"
)

type ClientHandler struct {
	repository    *slaverepositories.ClientRepository
	replicator    *replication.Replicator
	client        *http.Client
	mainMasterURL string
	selfName      string
	selfURL       string
	allNodes      []failover.NodeInfo
	logger        *log.Logger
}

func NewClientHandler(
	repository *slaverepositories.ClientRepository,
	replicator *replication.Replicator,
	mainMasterURL string,
	selfName string,
	selfURL string,
	allNodes []failover.NodeInfo,
	logger *log.Logger,
) *ClientHandler {
	if logger == nil {
		logger = log.Default()
	}

	return &ClientHandler{
		repository:    repository,
		replicator:    replicator,
		mainMasterURL: mainMasterURL,
		selfName:      selfName,
		selfURL:       selfURL,
		allNodes:      allNodes,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		logger: logger,
	}
}

func (h *ClientHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"node":   h.selfName,
		"role":   "Slave",
		"status": "online",
		"url":    h.selfURL,
	})
}

func (h *ClientHandler) ListClients(c *gin.Context) {
	clients, err := h.repository.List(c.Request.Context())
	if err != nil {
		h.logger.Printf("slave list clients failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list clients"})
		return
	}

	if htmlview.WantsHTML(c) {
		htmlview.RenderClients(c, clients, h.selfName)
		return
	}

	c.JSON(http.StatusOK, clients)
}

func (h *ClientHandler) ForwardWrite(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	approval, err := h.requestMasterApproval(c, body)
	if err == nil && approval.Pending {
		c.JSON(http.StatusAccepted, gin.H{
			"approved":   false,
			"pending":    true,
			"request_id": approval.RequestID,
			"message":    approval.Message,
		})
		return
	}
	if err == nil && !approval.Approved {
		h.logger.Printf("Master rejected write")
		c.JSON(http.StatusForbidden, gin.H{
			"approved": false,
			"message":  approval.Message,
		})
		return
	}

	// Try the main master first.
	status, responseBody, contentType, err := h.forwardTo(c, h.mainMasterURL, body)
	if err == nil {
		h.logger.Printf("Write forwarded after approval")
		c.Data(status, contentType, responseBody)
		return
	}

	h.logger.Printf("main master unavailable: %v, running failover election", err)

	// Dynamic election: find the closest online node to the failed master.
	elected := failover.ElectMaster(h.mainMasterURL, h.allNodes, func(url string) bool {
		if url == h.selfURL {
			return true // self is always considered online
		}
		return failover.CheckHealth(url)
	})

	if elected == nil {
		h.logger.Printf("no online node available for failover")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no master node available"})
		return
	}

	// If elected node is self, handle the write locally.
	if elected.Name == h.selfName {
		h.logger.Printf("elected self (%s) as temporary master", h.selfName)
		h.handleAsTemporaryMaster(c, body)
		return
	}

	// Forward to the elected node.
	h.logger.Printf("forwarding write to elected master: %s at %s", elected.Name, elected.URL)
	status, responseBody, contentType, err = h.forwardTo(c, elected.URL, body)
	if err == nil {
		c.Data(status, contentType, responseBody)
		return
	}

	h.logger.Printf("elected master %s also unavailable: %v", elected.Name, err)
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no master node available"})
}

func (h *ClientHandler) requestMasterApproval(c *gin.Context, body []byte) (models.ApproveWriteResponse, error) {
	payload, err := buildApprovalRequest(c, h.selfName, body)
	if err != nil {
		return models.ApproveWriteResponse{
			Approved: false,
			Message:  err.Error(),
		}, nil
	}

	h.logger.Printf("Slave requested approval from Master")

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return models.ApproveWriteResponse{}, err
	}

	request, err := http.NewRequestWithContext(
		c.Request.Context(),
		http.MethodPost,
		h.mainMasterURL+"/approve-write",
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return models.ApproveWriteResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := h.client.Do(request)
	if err != nil {
		return models.ApproveWriteResponse{}, err
	}
	defer response.Body.Close()

	var approvalResponse models.ApproveWriteResponse
	if err := json.NewDecoder(response.Body).Decode(&approvalResponse); err != nil {
		return models.ApproveWriteResponse{}, err
	}

	return approvalResponse, nil
}

func (h *ClientHandler) Replicate(c *gin.Context) {
	var request models.ReplicationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Printf("invalid replication request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.Client.ID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client.id must be a positive integer"})
		return
	}

	// Apply exactly the operation sent by the master node.
	var err error
	switch request.Operation {
	case operationInsert:
		err = h.repository.Insert(c.Request.Context(), request.Client)
	case operationUpdate:
		err = h.repository.Update(c.Request.Context(), request.Client)
	case operationDelete:
		err = h.repository.Delete(c.Request.Context(), request.Client.ID)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported replication operation"})
		return
	}

	if errors.Is(err, slaverepositories.ErrClientNotFound) {
		h.logger.Printf("replication skipped missing client operation=%s client_id=%d", request.Operation, request.Client.ID)
		c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		return
	}
	if err != nil {
		h.logger.Printf("replication failed operation=%s client_id=%d: %v", request.Operation, request.Client.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Printf("replication applied operation=%s client_id=%d", request.Operation, request.Client.ID)
	c.JSON(http.StatusOK, gin.H{"status": "replicated"})
}

func (h *ClientHandler) forwardTo(c *gin.Context, baseURL string, body []byte) (int, []byte, string, error) {
	url := baseURL + c.Request.URL.RequestURI()
	request, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, "", err
	}
	request.Header.Set("Content-Type", c.GetHeader("Content-Type"))

	response, err := h.client.Do(request)
	if err != nil {
		return 0, nil, "", err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, nil, "", err
	}

	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}

	h.logger.Printf("forwarded %s %s to %s with status=%d", c.Request.Method, c.Request.URL.RequestURI(), baseURL, response.StatusCode)
	return response.StatusCode, responseBody, contentType, nil
}

func (h *ClientHandler) handleAsTemporaryMaster(c *gin.Context, body []byte) {
	switch c.Request.Method {
	case http.MethodPost:
		h.createDuringFailover(c, body)
	case http.MethodPut:
		h.updateDuringFailover(c, body)
	case http.MethodDelete:
		h.deleteDuringFailover(c)
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}

func (h *ClientHandler) createDuringFailover(c *gin.Context, body []byte) {
	var request models.CreateClientRequest
	if !decodeAndValidate(c, body, &request) {
		return
	}

	client, err := h.repository.Create(c.Request.Context(), request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.replicator.Replicate(replication.OperationInsert, client)
	c.JSON(http.StatusCreated, client)
}

func (h *ClientHandler) updateDuringFailover(c *gin.Context, body []byte) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	var request models.UpdateClientRequest
	if !decodeAndValidate(c, body, &request) {
		return
	}

	client := models.Client{
		ID:         id,
		Name:       request.Name,
		NationalID: request.NationalID,
		Phone:      request.Phone,
		Email:      request.Email,
		Balance:    request.Balance,
	}
	err := h.repository.Update(c.Request.Context(), client)
	if errors.Is(err, slaverepositories.ErrClientNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.replicator.Replicate(replication.OperationUpdate, client)
	c.JSON(http.StatusOK, client)
}

func (h *ClientHandler) deleteDuringFailover(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	client, err := h.repository.GetByID(c.Request.Context(), id)
	if errors.Is(err, slaverepositories.ErrClientNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get client"})
		return
	}

	err = h.repository.Delete(c.Request.Context(), id)
	if errors.Is(err, slaverepositories.ErrClientNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete client"})
		return
	}

	h.replicator.Replicate(replication.OperationDelete, client)
	c.Status(http.StatusNoContent)
}

func decodeAndValidate(c *gin.Context, body []byte, request any) bool {
	if err := json.Unmarshal(body, request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}
	if err := binding.Validator.ValidateStruct(request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}

	return true
}

func parseIDParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be a positive integer"})
		return 0, false
	}

	return id, true
}

func buildApprovalRequest(c *gin.Context, sourceNode string, body []byte) (models.ApproveWriteRequest, error) {
	request := models.ApproveWriteRequest{
		SourceNode: sourceNode,
	}

	switch c.Request.Method {
	case http.MethodPost:
		request.Operation = "create"
		var createRequest models.CreateClientRequest
		if !jsonPayloadValid(body, &createRequest) {
			return request, errors.New("request body must be valid JSON")
		}
		request.Client = createRequest
	case http.MethodPut:
		request.Operation = "update"
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			return request, errors.New("id must be a positive integer")
		}
		request.ClientID = id
		var updateRequest models.UpdateClientRequest
		if !jsonPayloadValid(body, &updateRequest) {
			return request, errors.New("request body must be valid JSON")
		}
		request.Client = models.CreateClientRequest(updateRequest)
	case http.MethodDelete:
		request.Operation = "delete"
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			return request, errors.New("id must be a positive integer")
		}
		request.ClientID = id
	default:
		return request, errors.New("method not allowed")
	}

	return request, nil
}

func jsonPayloadValid(body []byte, request any) bool {
	return json.Unmarshal(body, request) == nil
}
