package storage

import (
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
	StatusChain   string  `json:"status_chain"`
	RequestBody   string  `json:"request_body,omitempty"`
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
	sessions      = make(map[string]*RecoverySession)
	sessionMutex  sync.RWMutex
	sessionExpire int
	logKeepDays   int
)

func Init(expireMinutes, keepDays int) {
	sessionExpire = expireMinutes
	logKeepDays = keepDays

	go cleanExpiredSessions()
	go cleanExpiredLogs()
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

// AddLog 追加写入一条请求日志到 SQLite。
func AddLog(entry LogEntry) {
	recoverInt := 0
	if entry.Recover {
		recoverInt = 1
	}
	DB.Exec(`INSERT INTO request_logs
		(request_id, client_ip, request_time, cost, status, status_chain, request_body,
		 recover, recover_count, error, error_phase, retry_count, partial_output, result)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.RequestID, entry.ClientIP, entry.RequestTime, entry.Cost, entry.HTTPStatus,
		entry.StatusChain, entry.RequestBody, recoverInt, entry.RecoverCount, entry.Error,
		entry.ErrorPhase, entry.RetryCount, entry.PartialOutput, entry.Result)
}

// GetLogs 返回最近的请求日志（新的在前）。
func GetLogs(c *gin.Context) {
	c.JSON(200, queryRequestLogs(1000))
}

func queryRequestLogs(limit int) []LogEntry {
	var result []LogEntry
	rows, err := DB.Query(`SELECT request_id, client_ip, request_time, cost, status, status_chain,
		request_body, recover, recover_count, error, error_phase, retry_count, partial_output, result
		FROM request_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return []LogEntry{}
	}
	defer rows.Close()

	for rows.Next() {
		var e LogEntry
		var recoverInt int
		if err := rows.Scan(&e.RequestID, &e.ClientIP, &e.RequestTime, &e.Cost, &e.HTTPStatus,
			&e.StatusChain, &e.RequestBody, &recoverInt, &e.RecoverCount, &e.Error,
			&e.ErrorPhase, &e.RetryCount, &e.PartialOutput, &e.Result); err != nil {
			continue
		}
		e.Recover = recoverInt == 1
		result = append(result, e)
	}
	return result
}

func ClearLogs(c *gin.Context) {
	DB.Exec(`DELETE FROM request_logs`)
	DB.Exec(`DELETE FROM upstream_logs`)
	c.JSON(200, map[string]string{"message": "Logs cleared"})
}

// AddUpstreamLog 追加写入一条上游日志到 SQLite。
func AddUpstreamLog(entry UpstreamLogEntry) {
	streamInt := 0
	if entry.Stream {
		streamInt = 1
	}
	DB.Exec(`INSERT INTO upstream_logs
		(request_id, client_ip, request_time, cost, status, model, stream, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.RequestID, entry.ClientIP, entry.RequestTime, entry.Cost,
		entry.HTTPStatus, entry.Model, streamInt, entry.Error)
}

// GetUpstreamLogs 返回最近的上游日志（新的在前）。
func GetUpstreamLogs(c *gin.Context) {
	c.JSON(200, GetUpstreamLogEntries(1000))
}

// GetUpstreamLogEntries 返回最近的上游日志切片（内部用）。
func GetUpstreamLogEntries(limit int) []UpstreamLogEntry {
	var result []UpstreamLogEntry
	rows, err := DB.Query(`SELECT request_id, client_ip, request_time, cost, status, model, stream, error
		FROM upstream_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return []UpstreamLogEntry{}
	}
	defer rows.Close()

	for rows.Next() {
		var e UpstreamLogEntry
		var streamInt int
		if err := rows.Scan(&e.RequestID, &e.ClientIP, &e.RequestTime, &e.Cost,
			&e.HTTPStatus, &e.Model, &streamInt, &e.Error); err != nil {
			continue
		}
		e.Stream = streamInt == 1
		result = append(result, e)
	}
	return result
}

func cleanExpiredLogs() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().AddDate(0, 0, -logKeepDays)
		cutoffStr := cutoff.Format("2006-01-02 15:04:05")

		DB.Exec(`DELETE FROM request_logs WHERE request_time < ?`, cutoffStr)
		DB.Exec(`DELETE FROM upstream_logs WHERE request_time < ?`, cutoffStr)
		cleanAPIUsage(cutoff)
	}
}

