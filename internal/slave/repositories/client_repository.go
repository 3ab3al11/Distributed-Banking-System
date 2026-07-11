package repositories

import (
	"context"
	"errors"
	"sort"
	"sync"

	"gin-sqlserver-api/internal/models"
)

var ErrClientNotFound = errors.New("client not found")

type ClientRepository struct {
	mu      sync.RWMutex
	clients map[int]models.Client
	nextID  int
}

func NewClientRepository() *ClientRepository {
	return &ClientRepository{
		clients: make(map[int]models.Client),
		nextID:  1,
	}
}

func (r *ClientRepository) List(ctx context.Context) ([]models.Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clients := make([]models.Client, 0, len(r.clients))
	for _, c := range r.clients {
		clients = append(clients, c)
	}

	sort.Slice(clients, func(i, j int) bool {
		return clients[i].ID < clients[j].ID
	})

	return clients, nil
}

func (r *ClientRepository) GetByID(ctx context.Context, id int) (models.Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	client, ok := r.clients[id]
	if !ok {
		return models.Client{}, ErrClientNotFound
	}

	return client, nil
}

func (r *ClientRepository) Create(ctx context.Context, request models.CreateClientRequest) (models.Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Enforce NationalId uniqueness
	for _, c := range r.clients {
		if c.NationalID == request.NationalID {
			return models.Client{}, errors.New("NationalId must be unique")
		}
	}

	client := models.Client{
		ID:         r.nextID,
		Name:       request.Name,
		NationalID: request.NationalID,
		Phone:      request.Phone,
		Email:      request.Email,
		Balance:    request.Balance,
	}
	r.clients[r.nextID] = client
	r.nextID++

	return client, nil
}

func (r *ClientRepository) Insert(ctx context.Context, client models.Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If the master sent an ID >= nextID, advance nextID to avoid future local ID conflicts
	if client.ID >= r.nextID {
		r.nextID = client.ID + 1
	}

	r.clients[client.ID] = client
	return nil
}

func (r *ClientRepository) Update(ctx context.Context, client models.Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.clients[client.ID]; !ok {
		return ErrClientNotFound
	}

	// Enforce NationalId uniqueness (excluding the current client)
	for _, c := range r.clients {
		if c.ID != client.ID && c.NationalID == client.NationalID {
			return errors.New("NationalId must be unique")
		}
	}

	r.clients[client.ID] = client
	return nil
}

func (r *ClientRepository) Delete(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.clients[id]; !ok {
		return ErrClientNotFound
	}

	delete(r.clients, id)
	return nil
}
