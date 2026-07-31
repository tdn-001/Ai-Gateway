package main

import (
	"ai-gateway/internal/auth"
	"ai-gateway/internal/config"
	"ai-gateway/internal/gateway"
	"ai-gateway/internal/logger"
	"ai-gateway/internal/storage"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	logger.Init()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	storage.Init(cfg.SessionExpireMinute, cfg.LogKeepDays)
	auth.Init()
	storage.LoadAPIKeys()
	storage.LoadAPIKeyUsage()

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/api/check-admin", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"exists": auth.IsAdminExists()})
	})

	v1 := r.Group("/v1")
	{
		v1.POST("/chat/completions", gateway.HandleChatCompletions)
	}

	admin := r.Group("/admin")
	{
		admin.POST("/login", auth.Login)
		admin.POST("/register", auth.Register)

		authorized := admin.Group("")
		authorized.Use(frontendNavigationMiddleware())
		authorized.Use(auth.AuthMiddleware())
		{
			authorized.GET("/config", config.GetConfig)
			authorized.PUT("/config", config.UpdateConfigHandler)
			authorized.GET("/prompts", config.GetPrompts)
			authorized.POST("/prompts", config.CreatePrompt)
			authorized.PUT("/prompts/:id", config.UpdatePrompt)
			authorized.DELETE("/prompts/:id", config.DeletePrompt)
			authorized.GET("/logs", storage.GetLogs)
			authorized.GET("/upstream-logs", storage.GetUpstreamLogs)
			authorized.DELETE("/logs", storage.ClearLogs)
			authorized.GET("/status", gateway.GetStatus)
			authorized.POST("/password", auth.ChangePassword)

			authorized.GET("/apikeys", func(c *gin.Context) {
				c.JSON(200, storage.GetAllAPIKeys())
			})
			authorized.POST("/apikeys", func(c *gin.Context) {
				var req struct {
					Name string `json:"name" binding:"required"`
					Key  string `json:"key"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}
				key := storage.CreateAPIKey(req.Name, req.Key)
				c.JSON(201, key)
			})
			authorized.DELETE("/apikeys/:key", func(c *gin.Context) {
				key := c.Param("key")
				if storage.DeleteAPIKey(key) {
					c.JSON(200, gin.H{"message": "Key deleted"})
				} else {
					c.JSON(404, gin.H{"error": "Key not found"})
				}
			})
			authorized.PUT("/apikeys/:key/toggle", func(c *gin.Context) {
				key := c.Param("key")
				var req struct {
					Enabled bool `json:"enabled"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}
				if storage.ToggleAPIKey(key, req.Enabled) {
					c.JSON(200, gin.H{"message": "Key updated"})
				} else {
					c.JSON(404, gin.H{"error": "Key not found"})
				}
			})
			authorized.GET("/apikeys/:key/usage", func(c *gin.Context) {
				key := c.Param("key")
				page := 1
				pageSize := 20
				if p := c.Query("page"); p != "" {
					fmt.Sscanf(p, "%d", &page)
				}
				if ps := c.Query("page_size"); ps != "" {
					fmt.Sscanf(ps, "%d", &pageSize)
				}
				logs, total := storage.GetAPIKeyUsageLogs(key, page, pageSize)
				c.JSON(200, gin.H{
					"logs":      logs,
					"total":     total,
					"page":      page,
					"page_size": pageSize,
				})
			})

			authorized.GET("/stats", func(c *gin.Context) {
				c.JSON(200, storage.GetTotalStats())
			})
			authorized.GET("/stats/active-ips", func(c *gin.Context) {
				c.JSON(200, storage.GetActiveIPs())
			})
			authorized.GET("/stats/trend", func(c *gin.Context) {
				interval := c.DefaultQuery("interval", "hour")
				hours := 24
				if h := c.Query("hours"); h != "" {
					fmt.Sscanf(h, "%d", &hours)
				}
				c.JSON(200, storage.GetRequestTrend(interval, hours))
			})
			authorized.GET("/location/:ip", func(c *gin.Context) {
				ip := c.Param("ip")
				location := gateway.GetIPLocation(ip)
				c.JSON(200, location)
			})

			authorized.GET("/modelkeys", func(c *gin.Context) {
				c.JSON(200, storage.GetAllModelKeys())
			})
			authorized.POST("/modelkeys", func(c *gin.Context) {
				var req struct {
					Name string `json:"name" binding:"required"`
					Key  string `json:"key" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}
				mk := storage.CreateModelKey(req.Name, req.Key)
				c.JSON(201, mk)
			})
			authorized.DELETE("/modelkeys/:key", func(c *gin.Context) {
				key := c.Param("key")
				if storage.DeleteModelKey(key) {
					c.JSON(200, gin.H{"message": "Key deleted"})
				} else {
					c.JSON(404, gin.H{"error": "Key not found"})
				}
			})
			authorized.PUT("/modelkeys/:key/toggle", func(c *gin.Context) {
				key := c.Param("key")
				var req struct {
					Enabled bool `json:"enabled"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}
				if storage.ToggleModelKey(key, req.Enabled) {
					c.JSON(200, gin.H{"message": "Key updated"})
				} else {
					c.JSON(404, gin.H{"error": "Key not found"})
				}
			})
		}
	}

	setupFrontend(r)

	addr := fmt.Sprintf(":%s", cfg.ListenPort)
	log.Printf("AI Gateway starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func init() {
	os.MkdirAll("./data", 0755)
}
