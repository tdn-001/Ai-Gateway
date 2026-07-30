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
	RequestID      string                 `json:"request_id"`
	ClientIP       string                 `json:"client_ip"`
	UserQuestion   string                 `json:"user_question"`
	OriginalRequest map[string]interface{} `json:"original_request"`
	PreviousOutput string                 `json:"previous_output"`
	RecoverMode    string                 `json:"recover_mode"`
	CreateTime     time.Time              `json:"create_time"`
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

var (
	sessions       = make(map[string]*RecoverySession)
	sessionMutex   sync.RWMutex
	logs           []LogEntry
	logMutex       sync.RWMutex
	sessionExpire  int
	logKeepDays    int
)

const (
	requestLogDir     = "./data/logs"
	requestLogPattern = "request_*.log"
	zapLogDir         = "./data/logs"
	zapLogPattern     = "gateway_*.log"
)

func Init(expireMinutes, keepDays int) {
	sessionExpire = expireMinutes
	logKeepDays = keepDays

	// 启动会话清理协程
	go cleanExpiredSessions()

	// 启动日志清理协程
	go cleanExpiredLogs()

	// 加载现有日志
	loadLogs()
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

// 日志管理
func AddLog(entry LogEntry) {
	logMutex.Lock()
	defer logMutex.Unlock()

	logs = append(logs, entry)
	saveLog(entry)
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
	defer logMutex.Unlock()

	logs = make([]LogEntry, 0)
	clearLogFiles()
	c.JSON(200, gin.H{"message": "Logs cleared"})
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

		cleanOldLogFiles(cutoff, requestLogDir, requestLogPattern)
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

func cleanAPIUsage(cutoff time.Time) {
	apiKeyUsageMu.Lock()
	defer apiKeyUsageMu.Unlock()

	var newUsage []APIKeyUsageLog
	for _, u := range apiKeyUsage {
		t, err := time.Parse("2006-01-02 15:04:05", u.RequestTime)
		if err != nil || t.After(cutoff) {
			newUsage = append(newUsage, u)
		}
	}
	apiKeyUsage = newUsage

	data, _ := json.MarshalIndent(apiKeyUsage, "", "  ")
	os.WriteFile(apiUsageFile, data, 0644)
}