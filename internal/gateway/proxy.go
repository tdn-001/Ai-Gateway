package gateway

import (
	"ai-gateway/internal/config"
	"ai-gateway/internal/logger"
	"ai-gateway/internal/storage"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ChatCompletionRequest struct {
	Model    string                   `json:"model"`
	Messages []map[string]interface{} `json:"messages"`
	Stream   bool                     `json:"stream"`
}

type ChatCompletionResponse struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
	Created int64                    `json:"created"`
	Model   string                   `json:"model"`
	Choices []map[string]interface{} `json:"choices"`
	Usage   map[string]interface{}   `json:"usage"`
}

func isRecoverableError(statusCode int) bool {
	return statusCode == 429 || statusCode == 500 || statusCode == 502 || statusCode == 503 || statusCode == 504
}

func HandleChatCompletions(c *gin.Context) {
	startTime := time.Now()
	requestID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	clientIP := c.ClientIP()

	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		apiKey = c.Query("api_key")
	}
	if apiKey == "" {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			apiKey = authHeader[7:]
		}
	}

	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing API Key. Please provide X-API-Key header, Authorization Bearer header, or api_key query parameter"})
		return
	}

	if !storage.ValidateAPIKey(apiKey) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or disabled API Key"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		logger.Error("Failed to parse request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load config"})
		return
	}

	userQuestion := ""
	for _, msg := range req.Messages {
		if role, ok := msg["role"].(string); ok && role == "user" {
			if content, ok := msg["content"].(string); ok {
				userQuestion = content
				break
			}
		}
	}

	session := &storage.RecoverySession{
		RequestID:       requestID,
		ClientIP:        clientIP,
		UserQuestion:    userQuestion,
		OriginalRequest: make(map[string]interface{}),
		PreviousOutput:  "",
		RecoverMode:     cfg.DefaultRecoveryMode,
		CreateTime:      time.Now(),
	}

	var originalReq map[string]interface{}
	json.Unmarshal(body, &originalReq)
	session.OriginalRequest = originalReq

	storage.CreateSession(session)

	maxRetries := 1
	if cfg.SSERecoveryEnable {
		maxRetries = cfg.MaxRetryTimes
	}

	var resp *http.Response
	totalRetries := 0
	lastError := ""

	for retryCount := 0; retryCount <= maxRetries; retryCount++ {
		if retryCount > 0 {
			totalRetries = retryCount
			logger.Info("Retrying request", zap.Int("retry", retryCount), zap.String("requestID", requestID))
			time.Sleep(time.Duration(1) * time.Second)
		}

		nginxURL := fmt.Sprintf("%s/v1/chat/completions", cfg.NginxUpstreamURL)

		var client *http.Client
		if req.Stream {
			client = &http.Client{}
		} else {
			client = &http.Client{
				Timeout: time.Duration(cfg.UpstreamTimeout) * time.Second,
			}
		}

		nginxReq, err := http.NewRequest("POST", nginxURL, bytes.NewReader(body))
		if err != nil {
			logger.Error("Failed to create nginx request", zap.Error(err))
			lastError = fmt.Sprintf("创建请求失败: %v", err)
			continue
		}

		for key, values := range c.Request.Header {
			for _, value := range values {
				if key != "X-API-Key" && key != "Authorization" {
					nginxReq.Header.Add(key, value)
				}
			}
		}
		nginxReq.Header.Set("Host", c.Request.Host)

		modelKey := storage.GetEnabledModelKey()
		if modelKey != nil {
			nginxReq.Header.Set("Authorization", "Bearer "+modelKey.Key)
		}

		resp, err = client.Do(nginxReq)
		if err != nil {
			logger.Error("Failed to send request to nginx", zap.Error(err))
			lastError = fmt.Sprintf("连接上游失败: %v", err)
			continue
		}

		if !isRecoverableError(resp.StatusCode) {
			break
		}

		lastError = fmt.Sprintf("上游返回可恢复错误: HTTP %d", resp.StatusCode)
		if retryCount < maxRetries {
			resp.Body.Close()
			logger.Warn("Got recoverable error, will retry", zap.Int("status", resp.StatusCode), zap.Int("retry", retryCount+1))
			continue
		}
		break
	}

	if resp == nil {
		storage.DeleteSession(requestID)
		logEntry := storage.LogEntry{
			RequestID:    requestID,
			ClientIP:     clientIP,
			RequestTime:  startTime.Format("2006-01-02 15:04:05"),
			Cost:         time.Since(startTime).Seconds(),
			HTTPStatus:   502,
			Error:        lastError,
			ErrorPhase:   "connect",
			RetryCount:   totalRetries,
		}
		storage.AddLog(logEntry)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to upstream"})
		return
	}
	defer resp.Body.Close()

	cost := time.Since(startTime).Seconds()
	logEntry := storage.LogEntry{
		RequestID:    requestID,
		ClientIP:     clientIP,
		RequestTime:  startTime.Format("2006-01-02 15:04:05"),
		Cost:         cost,
		HTTPStatus:   resp.StatusCode,
		Recover:      false,
		RecoverCount: 0,
		Error:        "",
		ErrorPhase:   "",
		RetryCount:   totalRetries,
		Result:       "",
	}

	storage.UpdateAPIKeyLastUsed(apiKey)
	storage.AddAPIKeyUsageLog(storage.APIKeyUsageLog{
		Key:         apiKey,
		RequestID:   requestID,
		ClientIP:    clientIP,
		RequestTime: startTime.Format("2006-01-02 15:04:05"),
		Cost:        cost,
		Status:      resp.StatusCode,
		Model:       req.Model,
	})

	if req.Stream {
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			logEntry.HTTPStatus = resp.StatusCode
			logEntry.Error = fmt.Sprintf("上游返回错误: HTTP %d", resp.StatusCode)
			logEntry.Result = string(body)
			if session.PreviousOutput != "" && cfg.SSERecoveryEnable {
				handleRecovery(c, session, &logEntry, 0)
			} else {
				c.Data(resp.StatusCode, "application/json", body)
			}
		} else {
			handleSSEStream(c, resp, session, &logEntry)
		}
	} else {
		handleJSONResponse(c, resp, session, &logEntry, cfg)
	}

	storage.AddLog(logEntry)
	storage.DeleteSession(requestID)
}

func handleJSONResponse(c *gin.Context, resp *http.Response, session *storage.RecoverySession, logEntry *storage.LogEntry, cfg *config.Config) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body", zap.Error(err))
		logEntry.Error = err.Error()
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to read response"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		logEntry.Error = fmt.Sprintf("Upstream returned status %d", resp.StatusCode)

		if cfg.SSERecoveryEnable && isRecoverableError(resp.StatusCode) {
			logger.Info("Got error response, attempting recovery", zap.Int("status", resp.StatusCode))
			if session.PreviousOutput == "" {
				for recoveryAttempt := 1; recoveryAttempt <= cfg.MaxRetryTimes; recoveryAttempt++ {
					logger.Info("Retrying request for error response", zap.Int("attempt", recoveryAttempt))
					recoveryResp, err := sendRecoveryRequest(cfg, session.OriginalRequest)
					if err != nil {
						logger.Error("Recovery request failed", zap.Error(err))
						continue
					}

					if recoveryResp.StatusCode == http.StatusOK {
						recoveryBody, _ := io.ReadAll(recoveryResp.Body)
						recoveryResp.Body.Close()
						var response ChatCompletionResponse
						if err := json.Unmarshal(recoveryBody, &response); err == nil && len(response.Choices) > 0 {
							logEntry.Recover = true
							logEntry.RecoverCount = recoveryAttempt
							logEntry.HTTPStatus = 200
							if message, ok := response.Choices[0]["message"].(map[string]interface{}); ok {
								if content, ok := message["content"].(string); ok {
									logEntry.Result = content
								}
							}
							c.Data(200, "application/json", recoveryBody)
							return
						}
					}
					recoveryResp.Body.Close()
				}
			}
		}

		c.Data(resp.StatusCode, "application/json", body)
		return
	}

	var response ChatCompletionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		logger.Error("Failed to parse response", zap.Error(err))
		logEntry.Error = err.Error()
		c.Data(resp.StatusCode, "application/json", body)
		return
	}

	if len(response.Choices) > 0 {
		if message, ok := response.Choices[0]["message"].(map[string]interface{}); ok {
			if content, ok := message["content"].(string); ok {
				logEntry.Result = content
				session.PreviousOutput = content
				storage.UpdateSession(session)
			}
		}
	}

	c.Data(resp.StatusCode, "application/json", body)
}

func GetStatus(c *gin.Context) {
	cfg, err := config.Load()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"status": "running",
		"config": cfg,
	})
}