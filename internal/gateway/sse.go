package gateway

import (
	"ai-gateway/internal/config"
	"ai-gateway/internal/logger"
	"ai-gateway/internal/storage"
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SSEEvent struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
	Created int64                    `json:"created"`
	Model   string                   `json:"model"`
	Choices []map[string]interface{} `json:"choices"`
	Usage   map[string]interface{}   `json:"usage,omitempty"`
}

type sseBuffer struct {
	lines  []string
	output strings.Builder
}

func (b *sseBuffer) reset() {
	b.lines = nil
	b.output.Reset()
}

func forwardSSEToClient(c *gin.Context, lines []string) {
	for _, line := range lines {
		c.Writer.Write([]byte(line + "\n\n"))
		c.Writer.Flush()
	}
}

func handleSSEStream(c *gin.Context, resp *http.Response, session *storage.RecoverySession, logEntry *storage.LogEntry, apiUsageLog *storage.APIKeyUsageLog, promptTokensEstimate int) {
	cfg, _ := config.Load()
	isBuffered := cfg != nil && cfg.BufferMode

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var buf sseBuffer
	var fullContent strings.Builder
	hasOutput := false
	var lastUsage map[string]interface{}

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
				logger.Error("Failed to parse SSE event", zap.Error(err))
				continue
			}

			if len(event.Choices) > 0 {
				if delta, ok := event.Choices[0]["delta"].(map[string]interface{}); ok {
					if content, ok := delta["content"].(string); ok {
						fullContent.WriteString(content)
						hasOutput = true
					}
				}
			}

			if event.Usage != nil && len(event.Usage) > 0 {
				lastUsage = event.Usage
			}

			if isBuffered {
				buf.lines = append(buf.lines, line)
			} else {
				c.Writer.Write([]byte(line + "\n\n"))
				c.Writer.Flush()
			}
		}
	}

	session.PreviousOutput = fullContent.String()
	storage.UpdateSession(session)
	logEntry.Result = fullContent.String()

	if lastUsage != nil {
		if promptTokens, ok := lastUsage["prompt_tokens"].(float64); ok {
			apiUsageLog.PromptTokens = int(promptTokens)
		}
		if completionTokens, ok := lastUsage["completion_tokens"].(float64); ok {
			apiUsageLog.CompletionTokens = int(completionTokens)
		}
		if totalTokens, ok := lastUsage["total_tokens"].(float64); ok {
			apiUsageLog.TotalTokens = int(totalTokens)
		}
	} else {
		// 部分上游（NVIDIA/OneAPI 等）流式响应即使请求 include_usage 也不返回用量，
		// 此时按请求/输出内容估算 token 数兜底，保证系统统计不为 0。
		apiUsageLog.PromptTokens = promptTokensEstimate
		apiUsageLog.CompletionTokens = estimateTokens([]byte(fullContent.String()))
	}
	storage.AddAPIKeyUsageLog(*apiUsageLog)

	scannerErr := scanner.Err()
	if scannerErr != nil {
		logger.Error("SSE scanner error", zap.Error(scannerErr))
		logEntry.Error = scannerErr.Error()
		if hasOutput {
			logEntry.ErrorPhase = "stream"
			logEntry.PartialOutput = fullContent.String()
		} else {
			logEntry.ErrorPhase = "stream_init"
		}
		buf.reset()

		if hasOutput {
			handleRecovery(c, session, logEntry, 0)
		} else {
			writeRecoveryTerminate(c, fmt.Sprintf("流式响应中断: %v", scannerErr))
		}
		return
	}

	if isBuffered && len(buf.lines) > 0 {
		forwardSSEToClient(c, buf.lines)
		c.Writer.Write([]byte("\n"))
		c.Writer.Flush()
		buf.reset()
	}
}
