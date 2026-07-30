package gateway

import (
	"ai-gateway/internal/config"
	"ai-gateway/internal/logger"
	"ai-gateway/internal/storage"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func handleRecovery(c *gin.Context, session *storage.RecoverySession, logEntry *storage.LogEntry, currentRetryCount int) {
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load config for recovery", zap.Error(err))
		return
	}

	if !cfg.SSERecoveryEnable {
		logger.Info("SSE recovery is disabled")
		return
	}

	if currentRetryCount >= cfg.MaxRetryTimes {
		logger.Error("Max retry times reached", zap.Int("max", cfg.MaxRetryTimes))
		logEntry.Error = "Max retry times reached"
		return
	}

	promptMode := fmt.Sprintf("mode_%s", strings.ToLower(session.RecoverMode))
	prompts := config.GetPromptsByMode(promptMode)
	if len(prompts) == 0 {
		logger.Error("No enabled prompt found for mode", zap.String("mode", promptMode))
		return
	}
	prompt := prompts[0]

	recoveryMessages := prepareRecoveryMessages(session, prompt.Content)

	recoveryRequest := map[string]interface{}{
		"model":    session.OriginalRequest["model"],
		"messages": recoveryMessages,
		"stream":   true,
	}

	recoveryResp, err := sendRecoveryRequest(cfg, recoveryRequest)
	if err != nil {
		logger.Error("Failed to send recovery request", zap.Error(err))
		logEntry.Error = err.Error()
		return
	}
	defer recoveryResp.Body.Close()

	if recoveryResp.StatusCode != http.StatusOK {
		logger.Error("Recovery request returned non-OK status", zap.Int("status", recoveryResp.StatusCode))
		if currentRetryCount+1 < cfg.MaxRetryTimes {
			handleRecovery(c, session, logEntry, currentRetryCount+1)
		}
		return
	}

	handleRecoveryResponse(c, recoveryResp, session, logEntry, currentRetryCount+1)
}

func prepareRecoveryMessages(session *storage.RecoverySession, promptTemplate string) []map[string]interface{} {
	prompt := strings.Replace(promptTemplate, "{{question}}", session.UserQuestion, -1)
	prompt = strings.Replace(prompt, "{{previous_output}}", session.PreviousOutput, -1)
	prompt = strings.Replace(prompt, "{{error}}", "", -1)
	prompt = strings.Replace(prompt, "{{model}}", fmt.Sprintf("%v", session.OriginalRequest["model"]), -1)

	var messages []map[string]interface{}

	if session.RecoverMode == "A" {
		if originalMessages, ok := session.OriginalRequest["messages"].([]interface{}); ok {
			for _, msg := range originalMessages {
				if msgMap, ok := msg.(map[string]interface{}); ok {
					messages = append(messages, msgMap)
				}
			}
		}
	}

	messages = append(messages, map[string]interface{}{
		"role":    "system",
		"content": prompt,
	})

	return messages
}

func sendRecoveryRequest(cfg *config.Config, request map[string]interface{}) (*http.Response, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}

	nginxURL := fmt.Sprintf("%s/v1/chat/completions", cfg.NginxUpstreamURL)
	req, err := http.NewRequest("POST", nginxURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	modelKey := storage.GetEnabledModelKey()
	if modelKey != nil {
		req.Header.Set("Authorization", "Bearer "+modelKey.Key)
	}

	return client.Do(req)
}

func handleRecoveryResponse(c *gin.Context, resp *http.Response, session *storage.RecoverySession, logEntry *storage.LogEntry, retryCount int) {
	cfg, _ := config.Load()
	isBuffered := cfg != nil && cfg.BufferMode

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var buf sseBuffer
	if !isBuffered {
		buf.reset()
	}

	var fullContent strings.Builder
	if session.RecoverMode == "A" {
		fullContent.WriteString(session.PreviousOutput)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			if data == "[DONE]" {
				if isBuffered {
					buf.lines = append(buf.lines, "data: [DONE]")
				} else {
					c.Writer.Write([]byte("data: [DONE]\n\n"))
					c.Writer.Flush()
				}
				break
			}

			var event SSEEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				logger.Error("Failed to parse recovery SSE event", zap.Error(err))
				continue
			}

			if len(event.Choices) > 0 {
				if delta, ok := event.Choices[0]["delta"].(map[string]interface{}); ok {
					if content, ok := delta["content"].(string); ok {
						fullContent.WriteString(content)
					}
				}
			}

			if isBuffered {
				buf.lines = append(buf.lines, line)
			} else {
				c.Writer.Write([]byte(line + "\n\n"))
				c.Writer.Flush()
			}
		}
	}

	scannerErr := scanner.Err()
	if scannerErr != nil {
		logger.Error("Recovery SSE scanner error", zap.Error(scannerErr))
		logEntry.Error = fmt.Sprintf("恢复请求失败: %v", scannerErr)
		logEntry.ErrorPhase = "recovery"

		cfg, _ := config.Load()
		if retryCount < cfg.MaxRetryTimes {
			handleRecovery(c, session, logEntry, retryCount)
		}
		buf.reset()
		return
	}

	session.PreviousOutput = fullContent.String()
	storage.UpdateSession(session)

	logEntry.Result = fullContent.String()
	logEntry.Recover = true
	logEntry.RecoverCount = retryCount

	if isBuffered && len(buf.lines) > 0 {
		forwardSSEToClient(c, buf.lines)
		c.Writer.Write([]byte("\n"))
		c.Writer.Flush()
		buf.reset()
	}
}
