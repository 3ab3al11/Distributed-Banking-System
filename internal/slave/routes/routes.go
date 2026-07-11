package routes

import (
	slavehandlers "gin-sqlserver-api/internal/slave/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, handler *slavehandlers.ClientHandler) {
	router.GET("/health", handler.HealthCheck)
	router.GET("/clients", handler.ListClients)
	router.POST("/clients", handler.ForwardWrite)
	router.PUT("/clients", handler.ForwardWrite)
	router.PUT("/clients/:id", handler.ForwardWrite)
	router.DELETE("/clients", handler.ForwardWrite)
	router.DELETE("/clients/:id", handler.ForwardWrite)

	// Compatibility aliases keep older demo calls working.
	router.GET("/users", handler.ListClients)
	router.POST("/users", handler.ForwardWrite)
	router.PUT("/users", handler.ForwardWrite)
	router.PUT("/users/:id", handler.ForwardWrite)
	router.DELETE("/users", handler.ForwardWrite)
	router.DELETE("/users/:id", handler.ForwardWrite)
	router.POST("/replicate", handler.Replicate)
}
