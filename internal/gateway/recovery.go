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
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// writeRecoveryTerminate 在恢复失败时向已开启的 SSE 连接补发错误事件和 [DONE]，
// 避免客户端一直挂起等待后续数据。
func writeRecoveryTerminate(c *gin.Context, errMsg string) {
	if errMsg == "" {
		errMsg = "recovery failed"
	}
	errData, _ := json.Marshal(map[string]interface{}{
		"error": map[string]interface{}{
			"message": errMsg,
			"type":    "server_error",
		},
	})
	c.Writer.Write([]byte("data: " + string(errData) + "\n\n"))
	c.Writer.Write([]byte("data: [DONE]\n\n"))
	c.Writer.Flush()
}

func handleRecovery(c *gin.Context, session *storage.RecoverySession, logEntry *storage.LogEntry, currentRetryCount int) {
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load config for recovery", zap.Error(err))
		return
	}

	if !cfg.SSERecoveryEnable {
		logger.Info("SSE recovery is disabled")
		writeRecoveryTerminate(c, "SSE recovery is disabled")
		return
	}

	if currentRetryCount >= cfg.MaxRetryTimes {
		logger.Error("Max retry times reached", zap.Int("max", cfg.MaxRetryTimes))
		logEntry.Error = "Max retry times reached"
		writeRecoveryTerminate(c, "Max retry times reached")
		return
	}

	// 缓冲模式下客户端未收到任何内容，恢复时必须完整重生成，
	// 否则"接续生成"会让客户端永远看不到开头部分。
	if cfg.BufferMode {
		session.RecoverMode = "B"
	}

	promptMode := fmt.Sprintf("mode_%s", strings.ToLower(session.RecoverMode))
	prompts := config.GetPromptsByMode(promptMode)
	if len(prompts) == 0 {
		logger.Error("No enabled prompt found for mode", zap.String("mode", promptMode))
		writeRecoveryTerminate(c, fmt.Sprintf("No enabled prompt found for mode %s", promptMode))
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
		writeRecoveryTerminate(c, fmt.Sprintf("恢复请求失败: %v", err))
		return
	}
	defer recoveryResp.Body.Close()

	if recoveryResp.StatusCode != http.StatusOK {
		logger.Error("Recovery request returned non-OK status", zap.Int("status", recoveryResp.StatusCode))
		if currentRetryCount+1 < cfg.MaxRetryTimes {
			handleRecovery(c, session, logEntry, currentRetryCount+1)
		} else {
			writeRecoveryTerminate(c, fmt.Sprintf("恢复请求返回错误: HTTP %d", recoveryResp.StatusCode))
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

	timeout := cfg.UpstreamTimeout
	if timeout <= 0 {
		timeout = 300
	}
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

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
		} else {
			writeRecoveryTerminate(c, fmt.Sprintf("恢复流式响应中断: %v", scannerErr))
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
