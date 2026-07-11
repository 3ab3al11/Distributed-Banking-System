package routes

import (
	"gin-sqlserver-api/internal/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterClientRoutes(router *gin.Engine, handler *handlers.ClientHandler) {
	router.GET("/health", handler.HealthCheck)
	router.GET("/clients", handler.ListClients)
	router.POST("/approve-write", handler.ApproveWrite)
	router.GET("/pending-writes", handler.ListPendingWrites)
	router.POST("/pending-writes/:id/approve", handler.ApprovePendingWrite)
	router.POST("/pending-writes/:id/reject", handler.RejectPendingWrite)
	router.POST("/clients", handler.CreateClient)
	router.PUT("/clients/:id", handler.UpdateClient)
	router.DELETE("/clients/:id", handler.DeleteClient)

	// Compatibility aliases keep older demo calls working.
	router.GET("/users", handler.ListClients)
	router.POST("/users", handler.CreateClient)
	router.PUT("/users/:id", handler.UpdateClient)
	router.DELETE("/users/:id", handler.DeleteClient)
}
