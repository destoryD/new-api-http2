package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func ResetGlobalProxyPoolRuntime(c *gin.Context) {
	service.ResetGlobalProxyPoolRuntime()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func GetGlobalProxyPoolStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    service.GetGlobalProxyPoolRuntimeStatus(),
	})
}
