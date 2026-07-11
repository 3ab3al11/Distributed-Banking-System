package main

import (
	"log"
	"os"

	"gin-sqlserver-api/internal/config"
	"gin-sqlserver-api/internal/failover"
	"gin-sqlserver-api/internal/replication"
	slavehandlers "gin-sqlserver-api/internal/slave/handlers"
	slaverepositories "gin-sqlserver-api/internal/slave/repositories"
	slaveroutes "gin-sqlserver-api/internal/slave/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	os.Setenv("NODE_NAME", "Slave1")
	cfg := config.LoadSlave()

	// In-memory RAM storage replaces Slave1DB.
	clientRepository := slaverepositories.NewClientRepository()

	// Slave1 replicates to all other slaves when acting as temporary master.
	replicator := replication.New([]string{
		cfg.Slave2URL + "/replicate",
		cfg.Slave3URL + "/replicate",
		cfg.Slave4URL + "/replicate",
	}, log.Default())

	// All cluster nodes for dynamic failover election.
	allNodes := []failover.NodeInfo{
		{Name: "Master", URL: cfg.MasterURL, Role: "Master"},
		{Name: "Slave1", URL: cfg.Slave1URL, Role: "Slave"},
		{Name: "Slave2", URL: cfg.Slave2URL, Role: "Slave"},
		{Name: "Slave3", URL: cfg.Slave3URL, Role: "Slave"},
		{Name: "Slave4", URL: cfg.Slave4URL, Role: "Slave"},
	}

	// All slaves are master-capable with dynamic failover election.
	clientHandler := slavehandlers.NewClientHandler(
		clientRepository,
		replicator,
		cfg.MasterURL, // mainMasterURL
		"Slave1",      // selfName
		cfg.Slave1URL, // selfURL
		allNodes,
		log.Default(),
	)

	// Slave API: read-only /clients plus /replicate for master updates.
	router := gin.Default()
	slaveroutes.RegisterRoutes(router, clientHandler)

	log.Printf("in-memory slave1 node listening on %s", cfg.ServerAddress)
	if err := router.Run(cfg.ServerAddress); err != nil {
		log.Fatalf("slave server failed: %v", err)
	}
}
