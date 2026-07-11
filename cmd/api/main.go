package main

import (
	"log"

	"gin-sqlserver-api/internal/config"
	"gin-sqlserver-api/internal/handlers"
	"gin-sqlserver-api/internal/replication"
	"gin-sqlserver-api/internal/repositories"
	"gin-sqlserver-api/internal/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	// In-memory RAM storage replaces MasterDB.
	clientRepository := repositories.NewClientRepository()

	// Master sends every successful write to all slave nodes.
	replicator := replication.New([]string{
		cfg.Slave1URL + "/replicate",
		cfg.Slave2URL + "/replicate",
		cfg.Slave3URL + "/replicate",
		cfg.Slave4URL + "/replicate",
	}, log.Default())
	clientHandler := handlers.NewClientHandler(clientRepository, replicator, "Master", cfg.MasterURL)

	// Public master API: GET, POST, PUT, DELETE /clients + GET /health.
	router := gin.Default()
	routes.RegisterClientRoutes(router, clientHandler)

	log.Printf("in-memory master server listening on %s", cfg.ServerAddress)
	if err := router.Run("0.0.0.0:8080"); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
