package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

type APIKey struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	CreatedAt    string `json:"created_at"`
	LastUsedAt   string `json:"last_used_at"`
	RequestCount int64  `json:"request_count"`
	Enabled      bool   `json:"enabled"`
}

type APIKeyUsageLog struct {
	Key              string  `json:"key"`
	RequestID        string  `json:"request_id"`
	ClientIP         string  `json:"client_ip"`
	RequestTime      string  `json:"request_time"`
	Cost             float64 `json:"cost"`
	Status           int     `json:"status"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	TotalTokens      int     `json:"total_tokens,omitempty"`
}

var (
	apiKeyCache   []APIKey
	apiKeyCacheMu sync.RWMutex
	apiKeysDirty  bool
	apiKeysMu     sync.Mutex
	stopKeyFlush  chan struct{}
)

const apiKeysFile = "./data/api_keys.json"

func init() {
	stopKeyFlush = make(chan struct{})
	go periodicAPIKeyFlush()
}

func periodicAPIKeyFlush() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			flushAPIKeysIfDirty()
		case <-stopKeyFlush:
			return
		}
	}
}

func flushAPIKeysIfDirty() {
	apiKeysMu.Lock()
	if !apiKeysDirty {
		apiKeysMu.Unlock()
		return
	}
	apiKeysDirty = false
	apiKeysMu.Unlock()

	apiKeyCacheMu.RLock()
	keys := make([]APIKey, len(apiKeyCache))
	copy(keys, apiKeyCache)
	apiKeyCacheMu.RUnlock()

	saveAPIKeysToFile(keys)
}

func markAPIKeysDirty() {
	apiKeysMu.Lock()
	apiKeysDirty = true
	apiKeysMu.Unlock()
}

func GenerateAPIKey() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "tdn_" + hex.EncodeToString(bytes)
}

func loadAPIKeysFromFile() []APIKey {
	if _, err := os.Stat(apiKeysFile); os.IsNotExist(err) {
		defaultKey := APIKey{
			Key:          GenerateAPIKey(),
			Name:         "默认Key",
			CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
			LastUsedAt:   "",
			RequestCount: 0,
			Enabled:      true,
		}
		keys := []APIKey{defaultKey}
		data, _ := json.MarshalIndent(keys, "", "  ")
		os.WriteFile(apiKeysFile, data, 0644)
		fmt.Printf("Generated default API Key: %s\n", defaultKey.Key)
		return keys
	}

	data, err := os.ReadFile(apiKeysFile)
	if err != nil {
		return []APIKey{}
	}

	var keys []APIKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return []APIKey{}
	}

	return keys
}

func saveAPIKeysToFile(keys []APIKey) error {
	data, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(apiKeysFile, data, 0644)
}

func reloadAPIKeyCache() {
	apiKeyCacheMu.Lock()
	apiKeyCache = loadAPIKeysFromFile()
	apiKeyCacheMu.Unlock()
}

func LoadAPIKeys() {
	reloadAPIKeyCache()
}

func ValidateAPIKey(key string) bool {
	apiKeyCacheMu.RLock()
	cached := apiKeyCache
	apiKeyCacheMu.RUnlock()
	for _, k := range cached {
		if k.Key == key && k.Enabled {
			return true
		}
	}
	return false
}

// UpdateAPIKeyLastUsed 只更新内存缓存，不落盘；由后台协程定期写入 api_keys.json。
func UpdateAPIKeyLastUsed(key string) {
	apiKeyCacheMu.Lock()
	defer apiKeyCacheMu.Unlock()
	for i, k := range apiKeyCache {
		if k.Key == key {
			apiKeyCache[i].LastUsedAt = time.Now().Format("2006-01-02 15:04:05")
			apiKeyCache[i].RequestCount++
			break
		}
	}
	markAPIKeysDirty()
}

func CreateAPIKey(name string, customKey string) APIKey {
	keyValue := customKey
	if keyValue == "" {
		keyValue = GenerateAPIKey()
	}

	newKey := APIKey{
		Key:          keyValue,
		Name:         name,
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
		LastUsedAt:   "",
		RequestCount: 0,
		Enabled:      true,
	}

	apiKeyCacheMu.Lock()
	apiKeyCache = append(apiKeyCache, newKey)
	apiKeyCacheMu.Unlock()
	markAPIKeysDirty()

	return newKey
}

func DeleteAPIKey(key string) bool {
	keys := loadAPIKeysFromFile()
	for i, k := range keys {
		if k.Key == key {
			keys = append(keys[:i], keys[i+1:]...)
			saveAPIKeysToFile(keys)
			reloadAPIKeyCache()
			return true
		}
	}
	return false
}

func ToggleAPIKey(key string, enabled bool) bool {
	keys := loadAPIKeysFromFile()
	for i, k := range keys {
		if k.Key == key {
			keys[i].Enabled = enabled
			saveAPIKeysToFile(keys)
			reloadAPIKeyCache()
			return true
		}
	}
	return false
}

func GetAllAPIKeys() []APIKey {
	apiKeyCacheMu.RLock()
	cached := make([]APIKey, len(apiKeyCache))
	copy(cached, apiKeyCache)
	apiKeyCacheMu.RUnlock()
	return cached
}

// AddAPIKeyUsageLog 记录一次 API 调用用量，写入 SQLite api_usage 表。
func AddAPIKeyUsageLog(log APIKeyUsageLog) {
	AddTokens(log.PromptTokens, log.CompletionTokens)
	AddDailyRequest(log.PromptTokens + log.CompletionTokens)

	DB.Exec(`INSERT INTO api_usage
		(key, request_id, client_ip, request_time, cost, status, model, prompt_tokens, completion_tokens, total_tokens)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.Key, log.RequestID, log.ClientIP, log.RequestTime, log.Cost, log.Status,
		log.Model, log.PromptTokens, log.CompletionTokens, log.TotalTokens)
}

func GetAPIKeyUsageLogs(key string, page, pageSize int) ([]APIKeyUsageLog, int) {
	var logs []APIKeyUsageLog

	var total int
	if key == "" {
		DB.QueryRow(`SELECT COUNT(*) FROM api_usage`).Scan(&total)
	} else {
		DB.QueryRow(`SELECT COUNT(*) FROM api_usage WHERE key = ?`, key).Scan(&total)
	}

	var rows *sql.Rows
	var err error
	if key == "" {
		rows, err = DB.Query(`SELECT key, request_id, client_ip, request_time, cost, status, model, prompt_tokens, completion_tokens, total_tokens
			FROM api_usage ORDER BY id DESC LIMIT ? OFFSET ?`, pageSize, (page-1)*pageSize)
	} else {
		rows, err = DB.Query(`SELECT key, request_id, client_ip, request_time, cost, status, model, prompt_tokens, completion_tokens, total_tokens
			FROM api_usage WHERE key = ? ORDER BY id DESC LIMIT ? OFFSET ?`, key, pageSize, (page-1)*pageSize)
	}
	if err != nil {
		return []APIKeyUsageLog{}, total
	}
	defer rows.Close()

	for rows.Next() {
		var log APIKeyUsageLog
		if err := rows.Scan(&log.Key, &log.RequestID, &log.ClientIP, &log.RequestTime, &log.Cost,
			&log.Status, &log.Model, &log.PromptTokens, &log.CompletionTokens, &log.TotalTokens); err == nil {
			logs = append(logs, log)
		}
	}

	return logs, total
}

func GetTotalStats() map[string]interface{} {
	todayStr := time.Now().Format("2006-01-02")

	// 总请求数/今日请求数来自 daily_stats（永久保留，不受日志过期清理影响）
	var totalRequests int64
	DB.QueryRow(`SELECT COALESCE(SUM(request_count), 0) FROM daily_stats`).Scan(&totalRequests)

	var todayRequests int64
	DB.QueryRow(`SELECT COALESCE(request_count, 0) FROM daily_stats WHERE date = ?`, todayStr).Scan(&todayRequests)

	keys := loadAPIKeysFromFile()
	totalKeys := len(keys)

	// 重试次数来自请求日志（按日志保留期清理）
	var totalRetries, todayRetries int64
	DB.QueryRow(`SELECT COALESCE(SUM(retry_count), 0) FROM request_logs`).Scan(&totalRetries)
	DB.QueryRow(`SELECT COALESCE(SUM(retry_count), 0) FROM request_logs WHERE request_time >= ?`, todayStr+" 00:00:00").Scan(&todayRetries)

	totalTokens, todayTokens := GetTokenStats()

	return map[string]interface{}{
		"total_requests": totalRequests,
		"today_requests": todayRequests,
		"total_keys":     totalKeys,
		"total_retries":  totalRetries,
		"today_retries":  todayRetries,
		"total_tokens":   formatTokenCount(totalTokens),
		"today_tokens":   formatTokenCount(todayTokens),
	}
}

func GetActiveIPs() []map[string]interface{} {
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	activeIPs := make([]map[string]interface{}, 0)

	rows, err := DB.Query(`SELECT client_ip, MAX(request_time) AS last_seen FROM api_usage GROUP BY client_ip`)
	if err != nil {
		return activeIPs
	}
	defer rows.Close()

	for rows.Next() {
		var ip, lastSeenStr string
		if err := rows.Scan(&ip, &lastSeenStr); err != nil {
			continue
		}
		lastSeen, err := time.Parse("2006-01-02 15:04:05", lastSeenStr)
		if err != nil {
			continue
		}
		activeIPs = append(activeIPs, map[string]interface{}{
			"ip":        ip,
			"last_seen": lastSeen.Format("2006-01-02 15:04:05"),
			"active":    lastSeen.After(fiveMinutesAgo),
		})
	}

	return activeIPs
}

// usageBucketAgg 缓存单个时间桶的聚合结果。
type usageBucketAgg struct {
	count  int64
	tokens int64
}

// queryUsageBuckets 按 prefixLen 分组统计 api_usage 中 [cutoff, now] 内的请求数与 token 数。
func queryUsageBuckets(cutoffPrefix string, prefixLen int) map[string]usageBucketAgg {
	agg := make(map[string]usageBucketAgg)

	rows, err := DB.Query(`SELECT substr(request_time, 1, ?), COUNT(*),
		COALESCE(SUM(prompt_tokens) + SUM(completion_tokens), 0)
		FROM api_usage
		WHERE substr(request_time, 1, ?) >= ?
		GROUP BY substr(request_time, 1, ?)`,
		prefixLen, prefixLen, cutoffPrefix, prefixLen)
	if err != nil {
		return agg
	}
	defer rows.Close()

	for rows.Next() {
		var bucket string
		var a usageBucketAgg
		if err := rows.Scan(&bucket, &a.count, &a.tokens); err != nil {
			continue
		}
		agg[bucket] = a
	}

	return agg
}

func GetRequestTrend(interval string, hours int, minutes int) []map[string]interface{} {
	now := time.Now()
	var trend []map[string]interface{}

	// 周/月视图从持久化的 daily_stats 读取，不受日志清理影响
	if interval == "week" || interval == "month" {
		return getLongTermTrend(interval, now)
	}

	// 小时/分钟/天视图从 api_usage 读取（短期数据，按日志保留期清理）
	switch interval {
	case "hour":
		agg := queryUsageBuckets(now.Add(-time.Duration(hours)*time.Hour).Format("2006-01-02 15"), 13)
		for i := hours; i >= 0; i-- {
			t := now.Add(-time.Duration(i) * time.Hour)
			count := agg[t.Format("2006-01-02 15")].count
			trend = append(trend, map[string]interface{}{
				"time":  t.Format("15:00"),
				"count": count,
			})
		}
	case "day":
		agg := queryUsageBuckets(now.AddDate(0, 0, -7).Format("2006-01-02"), 10)
		for i := 7; i >= 0; i-- {
			t := now.AddDate(0, 0, -i)
			count := agg[t.Format("2006-01-02")].count
			trend = append(trend, map[string]interface{}{
				"time":  t.Format("01-02"),
				"count": count,
			})
		}
	case "minute":
		agg := queryUsageBuckets(now.Add(-time.Duration(minutes)*time.Minute).Format("2006-01-02 15:04"), 16)
		for i := minutes; i >= 0; i-- {
			t := now.Add(-time.Duration(i) * time.Minute)
			count := agg[t.Format("2006-01-02 15:04")].count
			trend = append(trend, map[string]interface{}{
				"time":  t.Format("15:04"),
				"count": count,
			})
		}
	}

	return trend
}

// getLongTermTrend 从持久化 daily_stats 读取周/月趋势，不受 LogKeepDays 清理影响。
func getLongTermTrend(interval string, now time.Time) []map[string]interface{} {
	var trend []map[string]interface{}

	switch interval {
	case "week":
		for i := 4; i >= 0; i-- {
			weekEnd := now.AddDate(0, 0, -i*7)
			weekStart := weekEnd.AddDate(0, 0, -6)
			dailyCounts := GetDailyRequestCount(weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02"))
			count := int64(0)
			for _, c := range dailyCounts {
				count += c
			}
			trend = append(trend, map[string]interface{}{
				"time":  weekStart.Format("01-02") + "~" + weekEnd.Format("01-02"),
				"count": count,
			})
		}
	case "month":
		for i := 11; i >= 0; i-- {
			t := now.AddDate(0, -i, 0)
			startDate := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).Format("2006-01-02")
			endDate := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Format("2006-01-02")
			dailyCounts := GetDailyRequestCount(startDate, endDate)
			count := int64(0)
			for _, c := range dailyCounts {
				count += c
			}
			trend = append(trend, map[string]interface{}{
				"time":  t.Format("2006-01"),
				"count": count,
			})
		}
	}

	return trend
}

// cleanAPIUsage 清理过期的 API 用量记录。
func cleanAPIUsage(cutoff time.Time) {
	DB.Exec(`DELETE FROM api_usage WHERE request_time < ?`, cutoff.Format("2006-01-02 15:04:05"))
}

func formatTokenCount(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return strconv.FormatInt(n, 10)
}

// GetTokenTrend 返回 token 消耗趋势，与 GetRequestTrend 相同的时间桶，
// 数据来自 api_usage（短期）与 daily_stats（周/月，永久）。
// points 控制返回的数据点数量，用于前端指定精度（如1小时24点）。
func GetTokenTrend(interval string, hours int, minutes int, points int) []map[string]interface{} {
	now := time.Now()
	var trend []map[string]interface{}

	switch interval {
	case "hour":
		agg := queryUsageBuckets(now.Add(-time.Duration(hours)*time.Hour).Format("2006-01-02 15"), 13)
		for i := hours; i >= 0; i-- {
			t := now.Add(-time.Duration(i) * time.Hour)
			tokens := agg[t.Format("2006-01-02 15")].tokens
			trend = append(trend, map[string]interface{}{
				"time":  t.Format("15:00"),
				"count": tokens,
			})
		}
	case "minute":
		// 按 step 分桶采样，返回约 points 个数据点
		if points <= 0 {
			points = minutes
		}
		step := 1
		if points < minutes {
			step = minutes / points
		}
		agg := queryUsageBuckets(now.Add(-time.Duration(minutes)*time.Minute).Format("2006-01-02 15:04"), 16)
		for i := minutes; i >= 0; i -= step {
			t := now.Add(-time.Duration(i) * time.Minute)
			tokens := agg[t.Format("2006-01-02 15:04")].tokens
			trend = append(trend, map[string]interface{}{
				"time":  t.Format("15:04"),
				"count": tokens,
			})
		}
	case "day":
		agg := queryUsageBuckets(now.AddDate(0, 0, -7).Format("2006-01-02"), 10)
		for i := 7; i >= 0; i-- {
			t := now.AddDate(0, 0, -i)
			tokens := agg[t.Format("2006-01-02")].tokens
			trend = append(trend, map[string]interface{}{
				"time":  t.Format("01-02"),
				"count": tokens,
			})
		}
	case "week":
		return getTokenLongTermTrend("week", now)
	case "month":
		return getTokenLongTermTrend("month", now)
	}

	return trend
}

func getTokenLongTermTrend(interval string, now time.Time) []map[string]interface{} {
	var trend []map[string]interface{}

	switch interval {
	case "week":
		for i := 4; i >= 0; i-- {
			weekEnd := now.AddDate(0, 0, -i*7)
			weekStart := weekEnd.AddDate(0, 0, -6)
			dailyTokens := GetDailyTokenCount(weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02"))
			count := int64(0)
			for _, c := range dailyTokens {
				count += c
			}
			trend = append(trend, map[string]interface{}{
				"time":  weekStart.Format("01-02") + "~" + weekEnd.Format("01-02"),
				"count": count,
			})
		}
	case "month":
		for i := 11; i >= 0; i-- {
			t := now.AddDate(0, -i, 0)
			startDate := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).Format("2006-01-02")
			endDate := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Format("2006-01-02")
			dailyTokens := GetDailyTokenCount(startDate, endDate)
			count := int64(0)
			for _, c := range dailyTokens {
				count += c
			}
			trend = append(trend, map[string]interface{}{
				"time":  t.Format("2006-01"),
				"count": count,
			})
		}
	}

	return trend
}
