package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"gin-sqlserver-api/internal/htmlview"
	"gin-sqlserver-api/internal/models"
	"gin-sqlserver-api/internal/replication"
	"gin-sqlserver-api/internal/repositories"

	"github.com/gin-gonic/gin"
)

type ClientHandler struct {
	repository      *repositories.ClientRepository
	replicator      *replication.Replicator
	nodeName        string
	nodeURL         string
	pendingMu       sync.RWMutex
	pendingRequests map[int]models.PendingWriteRequest
	nextPendingID   int
}

func NewClientHandler(repository *repositories.ClientRepository, replicator *replication.Replicator, nodeName, nodeURL string) *ClientHandler {
	return &ClientHandler{
		repository:      repository,
		replicator:      replicator,
		nodeName:        nodeName,
		nodeURL:         nodeURL,
		pendingRequests: make(map[int]models.PendingWriteRequest),
		nextPendingID:   1,
	}
}

func (h *ClientHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"node":   h.nodeName,
		"role":   "Master",
		"status": "online",
		"url":    h.nodeURL,
	})
}

func (h *ClientHandler) ListClients(c *gin.Context) {
	clients, err := h.repository.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list clients"})
		return
	}

	if htmlview.WantsHTML(c) {
		htmlview.RenderClients(c, clients, h.nodeName)
		return
	}

	c.JSON(http.StatusOK, clients)
}

func (h *ClientHandler) ApproveWrite(c *gin.Context) {
	var request models.ApproveWriteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusOK, models.ApproveWriteResponse{
			Approved: false,
			Message:  err.Error(),
		})
		return
	}

	message := validateApprovalRequest(request)
	if message != "" {
		c.JSON(http.StatusOK, models.ApproveWriteResponse{
			Approved: false,
			Message:  message,
		})
		return
	}

	pendingRequest := h.addPendingRequest(request)
	c.JSON(http.StatusOK, models.ApproveWriteResponse{
		Approved:  false,
		Pending:   true,
		RequestID: pendingRequest.RequestID,
		Message:   "Write request is pending master approval",
	})
}

func (h *ClientHandler) ListPendingWrites(c *gin.Context) {
	h.pendingMu.RLock()
	requests := make([]models.PendingWriteRequest, 0, len(h.pendingRequests))
	for _, pending := range h.pendingRequests {
		requests = append(requests, pending)
	}
	h.pendingMu.RUnlock()

	sortPendingRequests(requests)
	c.JSON(http.StatusOK, requests)
}

func (h *ClientHandler) ApprovePendingWrite(c *gin.Context) {
	pendingRequest, ok := h.getPendingRequestByID(c)
	if !ok {
		return
	}

	if pendingRequest.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "write request is not pending"})
		return
	}

	client, statusCode, err := h.executePendingWrite(c, pendingRequest)
	if err != nil {
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	pendingRequest.Status = "approved"
	h.savePendingRequest(pendingRequest)

	response := gin.H{
		"message": "Pending write approved",
		"request": pendingRequest,
	}
	if client.ID > 0 {
		response["client"] = client
	}

	c.JSON(http.StatusOK, response)
}

func (h *ClientHandler) RejectPendingWrite(c *gin.Context) {
	pendingRequest, ok := h.getPendingRequestByID(c)
	if !ok {
		return
	}

	if pendingRequest.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "write request is not pending"})
		return
	}

	pendingRequest.Status = "rejected"
	h.savePendingRequest(pendingRequest)
	c.JSON(http.StatusOK, gin.H{
		"message": "Pending write rejected",
		"request": pendingRequest,
	})
}

func (h *ClientHandler) CreateClient(c *gin.Context) {
	var request models.CreateClientRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client, err := h.repository.Create(c.Request.Context(), request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create client"})
		return
	}

	// Every successful write on the master is copied to all slave nodes.
	h.replicator.Replicate(replication.OperationInsert, client)

	c.JSON(http.StatusCreated, client)
}

func (h *ClientHandler) UpdateClient(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	var request models.UpdateClientRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client, err := h.repository.Update(c.Request.Context(), id, request)
	if errors.Is(err, repositories.ErrClientNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update client"})
		return
	}

	// Replicate the final updated row to the slaves.
	h.replicator.Replicate(replication.OperationUpdate, client)

	c.JSON(http.StatusOK, client)
}

func (h *ClientHandler) DeleteClient(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	// Read the row before deleting it so the delete message still has client data.
	client, err := h.repository.GetByID(c.Request.Context(), id)
	if errors.Is(err, repositories.ErrClientNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get client"})
		return
	}

	err = h.repository.Delete(c.Request.Context(), id)
	if errors.Is(err, repositories.ErrClientNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "client not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete client"})
		return
	}

	// Tell slaves to delete the same client after the master delete succeeds.
	h.replicator.Replicate(replication.OperationDelete, client)

	c.Status(http.StatusNoContent)
}

func parseIDParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be a positive integer"})
		return 0, false
	}

	return id, true
}

func validateApprovalRequest(request models.ApproveWriteRequest) string {
	switch request.Operation {
	case "create":
		if err := validateCreateLikeRequest(request.Client); err != nil {
			return err.Error()
		}
	case "update":
		if request.ClientID <= 0 {
			return "client_id must be a positive integer"
		}
		if err := validateCreateLikeRequest(request.Client); err != nil {
			return err.Error()
		}
	case "delete":
		if request.ClientID <= 0 {
			return "client_id must be a positive integer"
		}
	default:
		return "operation must be create, update, or delete"
	}

	return ""
}

func validateCreateLikeRequest(request models.CreateClientRequest) error {
	if request.Name == "" {
		return errors.New("name is required")
	}
	if len(request.Name) > 100 {
		return errors.New("name must be at most 100 characters")
	}
	if request.NationalID == "" {
		return errors.New("national_id is required")
	}
	if len(request.NationalID) > 14 {
		return errors.New("national_id must be at most 14 characters")
	}
	if request.Phone == "" {
		return errors.New("phone is required")
	}
	if len(request.Phone) > 15 {
		return errors.New("phone must be at most 15 characters")
	}
	if request.Email == "" {
		return errors.New("email is required")
	}
	if len(request.Email) > 150 {
		return errors.New("email must be at most 150 characters")
	}
	if request.Balance < 0 {
		return errors.New("balance must be a non-negative number")
	}

	return nil
}

func (h *ClientHandler) addPendingRequest(request models.ApproveWriteRequest) models.PendingWriteRequest {
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()

	pendingRequest := models.PendingWriteRequest{
		RequestID:  h.nextPendingID,
		Operation:  request.Operation,
		SourceNode: request.SourceNode,
		ClientID:   request.ClientID,
		Client:     request.Client,
		Status:     "pending",
		CreatedAt:  time.Now().UTC(),
	}
	h.pendingRequests[pendingRequest.RequestID] = pendingRequest
	h.nextPendingID++

	return pendingRequest
}

func (h *ClientHandler) getPendingRequestByID(c *gin.Context) (models.PendingWriteRequest, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be a positive integer"})
		return models.PendingWriteRequest{}, false
	}

	h.pendingMu.RLock()
	pendingRequest, ok := h.pendingRequests[id]
	h.pendingMu.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "pending write request not found"})
		return models.PendingWriteRequest{}, false
	}

	return pendingRequest, true
}

func (h *ClientHandler) savePendingRequest(request models.PendingWriteRequest) {
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()
	h.pendingRequests[request.RequestID] = request
}

func (h *ClientHandler) executePendingWrite(c *gin.Context, request models.PendingWriteRequest) (models.Client, int, error) {
	switch request.Operation {
	case "create":
		client, err := h.repository.Create(c.Request.Context(), request.Client)
		if err != nil {
			return models.Client{}, http.StatusInternalServerError, err
		}
		h.replicator.Replicate(replication.OperationInsert, client)
		return client, http.StatusOK, nil
	case "update":
		client, err := h.repository.Update(c.Request.Context(), request.ClientID, models.UpdateClientRequest(request.Client))
		if errors.Is(err, repositories.ErrClientNotFound) {
			return models.Client{}, http.StatusNotFound, errors.New("client not found")
		}
		if err != nil {
			return models.Client{}, http.StatusInternalServerError, err
		}
		h.replicator.Replicate(replication.OperationUpdate, client)
		return client, http.StatusOK, nil
	case "delete":
		client, err := h.repository.GetByID(c.Request.Context(), request.ClientID)
		if errors.Is(err, repositories.ErrClientNotFound) {
			return models.Client{}, http.StatusNotFound, errors.New("client not found")
		}
		if err != nil {
			return models.Client{}, http.StatusInternalServerError, errors.New("failed to get client")
		}
		err = h.repository.Delete(c.Request.Context(), request.ClientID)
		if errors.Is(err, repositories.ErrClientNotFound) {
			return models.Client{}, http.StatusNotFound, errors.New("client not found")
		}
		if err != nil {
			return models.Client{}, http.StatusInternalServerError, errors.New("failed to delete client")
		}
		h.replicator.Replicate(replication.OperationDelete, client)
		return client, http.StatusOK, nil
	default:
		return models.Client{}, http.StatusBadRequest, errors.New("unsupported operation")
	}
}

func sortPendingRequests(requests []models.PendingWriteRequest) {
	for i := 0; i < len(requests); i++ {
		for j := i + 1; j < len(requests); j++ {
			left := requests[i]
			right := requests[j]
			if left.RequestID > right.RequestID {
				requests[i], requests[j] = requests[j], requests[i]
			}
		}
	}
}
