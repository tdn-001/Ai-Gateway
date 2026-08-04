package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RecoverySession struct {
	RequestID       string                 `json:"request_id"`
	ClientIP        string                 `json:"client_ip"`
	UserQuestion    string                 `json:"user_question"`
	OriginalRequest map[string]interface{} `json:"original_request"`
	PreviousOutput  string                 `json:"previous_output"`
	RecoverMode     string                 `json:"recover_mode"`
	CreateTime      time.Time              `json:"create_time"`
	RecoveryKey     string                 `json:"recovery_key,omitempty"`
	RecoveryKeyFails int                   `json:"recovery_key_fails,omitempty"`
}

type LogEntry struct {
	RequestID     string  `json:"request_id"`
	ClientIP      string  `json:"client_ip"`
	RequestTime   string  `json:"request_time"`
	Cost          float64 `json:"cost"`
	HTTPStatus    int     `json:"status"`
	Recover       bool    `json:"recover"`
	RecoverCount  int     `json:"recover_count"`
	Error         string  `json:"error"`
	ErrorPhase    string  `json:"error_phase"`
	RetryCount    int     `json:"retry_count"`
	PartialOutput string  `json:"partial_output"`
	Result        string  `json:"result"`
}

type UpstreamLogEntry struct {
	RequestID   string  `json:"request_id"`
	ClientIP    string  `json:"client_ip"`
	RequestTime string  `json:"request_time"`
	Cost        float64 `json:"cost"`
	HTTPStatus  int     `json:"status"`
	Model       string  `json:"model"`
	Stream      bool    `json:"stream"`
	Error       string  `json:"error"`
}

var (
	sessions       = make(map[string]*RecoverySession)
	sessionMutex   sync.RWMutex
	logs           []LogEntry
	logMutex       sync.RWMutex
	upstreamLogs   []UpstreamLogEntry
	upstreamMutex  sync.RWMutex
	sessionExpire  int
	logKeepDays    int
	logChan        = make(chan LogEntry, 4096)
	upstreamLogCh  = make(chan UpstreamLogEntry, 4096)
	stopLogFlush   chan struct{}
)

const (
	requestLogDir     = "./data/logs"
	requestLogPattern = "request_*.log"
	upstreamLogDir    = "./data/logs"
	upstreamLogPattern = "upstream_*.log"
	zapLogDir         = "./data/logs"
	zapLogPattern     = "gateway_*.log"
)

func Init(expireMinutes, keepDays int) {
	sessionExpire = expireMinutes
	logKeepDays = keepDays

	stopLogFlush = make(chan struct{})
	go logFlushWorker()
	go cleanExpiredSessions()
	go cleanExpiredLogs()

	loadLogs()
	loadUpstreamLogs()
}

func logFlushWorker() {
	for {
		select {
		case entry := <-logChan:
			saveLog(entry)
		case entry := <-upstreamLogCh:
			saveUpstreamLog(entry)
		case <-stopLogFlush:
			return
		}
	}
}

// 会话管理
func CreateSession(session *RecoverySession) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	session.CreateTime = time.Now()
	sessions[session.RequestID] = session
}

func GetSession(requestID string) (*RecoverySession, bool) {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()

	session, exists := sessions[requestID]
	return session, exists
}

func UpdateSession(session *RecoverySession) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	sessions[session.RequestID] = session
}

func DeleteSession(requestID string) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	delete(sessions, requestID)
}

func cleanExpiredSessions() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sessionMutex.Lock()
		now := time.Now()
		for id, session := range sessions {
			if now.Sub(session.CreateTime) > time.Duration(sessionExpire)*time.Minute {
				delete(sessions, id)
			}
		}
		sessionMutex.Unlock()
	}
}

// AddLog 追加内存（同步，快速）+ 异步落盘，不阻塞请求关键路径。
func AddLog(entry LogEntry) {
	logMutex.Lock()
	logs = append(logs, entry)
	logMutex.Unlock()

	select {
	case logChan <- entry:
	default:
		// channel 满时丢弃异步写，内存已有记录，不会丢数据
	}
}

func GetLogs(c *gin.Context) {
	logMutex.RLock()
	defer logMutex.RUnlock()

	// 返回最近的日志
	start := 0
	if len(logs) > 1000 {
		start = len(logs) - 1000
	}
	c.JSON(200, logs[start:])
}

func ClearLogs(c *gin.Context) {
	logMutex.Lock()
	upstreamMutex.Lock()
	defer logMutex.Unlock()
	defer upstreamMutex.Unlock()

	logs = make([]LogEntry, 0)
	upstreamLogs = make([]UpstreamLogEntry, 0)
	clearLogFiles()
	clearUpstreamLogFiles()
	c.JSON(200, gin.H{"message": "Logs cleared"})
}

// AddUpstreamLog 追加内存（同步，快速）+ 异步落盘，不阻塞请求关键路径。
func AddUpstreamLog(entry UpstreamLogEntry) {
	upstreamMutex.Lock()
	upstreamLogs = append(upstreamLogs, entry)
	upstreamMutex.Unlock()

	select {
	case upstreamLogCh <- entry:
	default:
	}
}

func GetUpstreamLogs(c *gin.Context) {
	upstreamMutex.RLock()
	defer upstreamMutex.RUnlock()

	start := 0
	if len(upstreamLogs) > 1000 {
		start = len(upstreamLogs) - 1000
	}
	c.JSON(200, upstreamLogs[start:])
}

func saveUpstreamLog(entry UpstreamLogEntry) {
	os.MkdirAll(upstreamLogDir, 0755)
	logFileName := filepath.Join(upstreamLogDir, fmt.Sprintf("upstream_%s.log", time.Now().Format("2006-01-02")))

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	f, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	f.Write(data)
	f.WriteString("\n")
}

func loadUpstreamLogs() {
	upstreamMutex.Lock()
	defer upstreamMutex.Unlock()

	matches, err := filepath.Glob(filepath.Join(upstreamLogDir, upstreamLogPattern))
	if err != nil {
		return
	}

	for _, logFileName := range matches {
		data, err := os.ReadFile(logFileName)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var entry UpstreamLogEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
			upstreamLogs = append(upstreamLogs, entry)
		}
	}
}

func clearUpstreamLogFiles() {
	matches, err := filepath.Glob(filepath.Join(upstreamLogDir, upstreamLogPattern))
	if err != nil {
		return
	}

	for _, logFileName := range matches {
		os.Remove(logFileName)
	}
}

func saveLog(entry LogEntry) {
	logDir := requestLogDir
	os.MkdirAll(logDir, 0755)
	logFileName := filepath.Join(logDir, fmt.Sprintf("request_%s.log", time.Now().Format("2006-01-02")))
	
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	f, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	f.Write(data)
	f.WriteString("\n")
}

func loadLogs() {
	logMutex.Lock()
	defer logMutex.Unlock()

	matches, err := filepath.Glob(filepath.Join(requestLogDir, requestLogPattern))
	if err != nil {
		return
	}

	for _, logFileName := range matches {
		data, err := os.ReadFile(logFileName)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var entry LogEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
			logs = append(logs, entry)
		}
	}
}

func clearLogFiles() {
	matches, err := filepath.Glob(filepath.Join(requestLogDir, requestLogPattern))
	if err != nil {
		return
	}

	for _, logFileName := range matches {
		os.Remove(logFileName)
	}
}

func cleanExpiredLogs() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().AddDate(0, 0, -logKeepDays)

		logMutex.Lock()
		var newLogs []LogEntry
		for _, entry := range logs {
			t, err := time.Parse("2006-01-02 15:04:05", entry.RequestTime)
			if err != nil || t.After(cutoff) {
				newLogs = append(newLogs, entry)
			}
		}
		logs = newLogs
		logMutex.Unlock()

		upstreamMutex.Lock()
		var newUpstream []UpstreamLogEntry
		for _, entry := range upstreamLogs {
			t, err := time.Parse("2006-01-02 15:04:05", entry.RequestTime)
			if err != nil || t.After(cutoff) {
				newUpstream = append(newUpstream, entry)
			}
		}
		upstreamLogs = newUpstream
		upstreamMutex.Unlock()

		cleanOldLogFiles(cutoff, requestLogDir, requestLogPattern)
		cleanOldLogFiles(cutoff, upstreamLogDir, upstreamLogPattern)
		cleanOldLogFiles(cutoff, zapLogDir, zapLogPattern)
		cleanAPIUsage(cutoff)
	}
}

func cleanOldLogFiles(cutoff time.Time, dir string, pattern string) {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return
	}

	for _, logFileName := range matches {
		info, err := os.Stat(logFileName)
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			os.Remove(logFileName)
		}
	}
}

