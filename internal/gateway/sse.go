package gateway

import (
	"ai-gateway/internal/config"
	"ai-gateway/internal/logger"
	"ai-gateway/internal/storage"
	"bufio"
	"encoding/json"
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

func handleSSEStream(c *gin.Context, resp *http.Response, session *storage.RecoverySession, logEntry *storage.LogEntry) {
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
	recoverCount := 0
	hasOutput := false

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

		if hasOutput {
			handleRecovery(c, session, logEntry, recoverCount)
		}
		buf.reset()
		return
	}

	if isBuffered && len(buf.lines) > 0 {
		forwardSSEToClient(c, buf.lines)
		c.Writer.Write([]byte("\n"))
		c.Writer.Flush()
		buf.reset()
	}
}
