package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	Key         string  `json:"key"`
	RequestID   string  `json:"request_id"`
	ClientIP    string  `json:"client_ip"`
	RequestTime string  `json:"request_time"`
	Cost        float64 `json:"cost"`
	Status      int     `json:"status"`
	Model       string  `json:"model"`
}

var (
	apiKeyCache   []APIKey
	apiKeyCacheMu sync.RWMutex
	apiKeyUsage   []APIKeyUsageLog
	apiKeyUsageMu sync.RWMutex
	apiKeysDirty  bool
	apiKeysMu     sync.Mutex
	stopKeyFlush  chan struct{}
)

const (
	apiKeysFile  = "./data/api_keys.json"
	apiUsageFile = "./data/api_usage.jsonl"
)

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
	keys := loadAPIKeysFromFile()

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

	keys = append(keys, newKey)
	saveAPIKeysToFile(keys)
	reloadAPIKeyCache()
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

// LoadAPIKeyUsage 加载使用日志，兼容旧 json 与新 jsonl 两种格式。
func LoadAPIKeyUsage() {
	apiKeyUsageMu.Lock()
	defer apiKeyUsageMu.Unlock()

	// 优先加载 jsonl（新格式）
	if _, err := os.Stat(apiUsageFile); err == nil {
		data, err := os.ReadFile(apiUsageFile)
		if err != nil {
			return
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var entry APIKeyUsageLog
			if err := json.Unmarshal([]byte(line), &entry); err == nil {
				apiKeyUsage = append(apiKeyUsage, entry)
			}
		}
		return
	}

	// 迁移：旧格式 api_usage.json → api_usage.jsonl
	oldFile := "./data/api_usage.json"
	if _, err := os.Stat(oldFile); err == nil {
		data, err := os.ReadFile(oldFile)
		if err == nil {
			json.Unmarshal(data, &apiKeyUsage)
		}
		// 重写为 jsonl
		os.MkdirAll(filepath.Dir(apiUsageFile), 0755)
		f, ferr := os.OpenFile(apiUsageFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if ferr == nil {
			for _, entry := range apiKeyUsage {
				d, _ := json.Marshal(entry)
				f.Write(d)
				f.WriteString("\n")
			}
			f.Close()
		}
		os.Remove(oldFile)
	}
}

// AddAPIKeyUsageLog 追加写入内存 + 追加写入 jsonl 文件（O_APPEND），
// 不再每次全量重写整个文件。
func AddAPIKeyUsageLog(log APIKeyUsageLog) {
	apiKeyUsageMu.Lock()
	apiKeyUsage = append(apiKeyUsage, log)
	apiKeyUsageMu.Unlock()

	os.MkdirAll(filepath.Dir(apiUsageFile), 0755)
	data, _ := json.Marshal(log)
	f, err := os.OpenFile(apiUsageFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(data)
	f.WriteString("\n")
}

func GetAPIKeyUsageLogs(key string, page, pageSize int) ([]APIKeyUsageLog, int) {
	apiKeyUsageMu.RLock()
	defer apiKeyUsageMu.RUnlock()

	var filtered []APIKeyUsageLog
	for _, log := range apiKeyUsage {
		if key == "" || log.Key == key {
			filtered = append(filtered, log)
		}
	}

	total := len(filtered)
	start := (page - 1) * pageSize
	if start >= total {
		return []APIKeyUsageLog{}, total
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	return filtered[start:end], total
}

func GetTotalStats() map[string]interface{} {
	apiKeyUsageMu.RLock()
	totalRequests := len(apiKeyUsage)
	apiKeyUsageMu.RUnlock()

	keys := loadAPIKeysFromFile()
	totalKeys := len(keys)

	logMutex.RLock()
	totalRetries := 0
	for _, l := range logs {
		totalRetries += l.RetryCount
	}
	logMutex.RUnlock()

	return map[string]interface{}{
		"total_requests": totalRequests,
		"total_keys":     totalKeys,
		"total_retries":  totalRetries,
	}
}

func GetActiveIPs() []map[string]interface{} {
	apiKeyUsageMu.RLock()
	defer apiKeyUsageMu.RUnlock()

	ipMap := make(map[string]time.Time)
	for _, log := range apiKeyUsage {
		logTime, err := time.Parse("2006-01-02 15:04:05", log.RequestTime)
		if err != nil {
			continue
		}
		if existing, exists := ipMap[log.ClientIP]; !exists || logTime.After(existing) {
			ipMap[log.ClientIP] = logTime
		}
	}

	var activeIPs []map[string]interface{}
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)

	for ip, lastSeen := range ipMap {
		activeIPs = append(activeIPs, map[string]interface{}{
			"ip":        ip,
			"last_seen": lastSeen.Format("2006-01-02 15:04:05"),
			"active":    lastSeen.After(fiveMinutesAgo),
		})
	}

	return activeIPs
}

func GetRequestTrend(interval string, hours int) []map[string]interface{} {
	apiKeyUsageMu.RLock()
	defer apiKeyUsageMu.RUnlock()

	now := time.Now()
	var trend []map[string]interface{}

	switch interval {
	case "hour":
		for i := hours; i >= 0; i-- {
			t := now.Add(-time.Duration(i) * time.Hour)
			count := 0
			for _, log := range apiKeyUsage {
				logTime, err := time.Parse("2006-01-02 15:04:05", log.RequestTime)
				if err != nil {
					continue
				}
				if logTime.Hour() == t.Hour() && logTime.Day() == t.Day() && logTime.Month() == t.Month() {
					count++
				}
			}
			trend = append(trend, map[string]interface{}{
				"time":  t.Format("15:00"),
				"count": count,
			})
		}
	case "day":
		for i := 7; i >= 0; i-- {
			t := now.AddDate(0, 0, -i)
			count := 0
			for _, log := range apiKeyUsage {
				logTime, err := time.Parse("2006-01-02 15:04:05", log.RequestTime)
				if err != nil {
					continue
				}
				if logTime.Year() == t.Year() && logTime.YearDay() == t.YearDay() {
					count++
				}
			}
			trend = append(trend, map[string]interface{}{
				"time":  t.Format("01-02"),
				"count": count,
			})
		}
	case "week":
		for i := 4; i >= 0; i-- {
			t := now.AddDate(0, 0, -i*7)
			count := 0
			for _, log := range apiKeyUsage {
				logTime, err := time.Parse("2006-01-02 15:04:05", log.RequestTime)
				if err != nil {
					continue
				}
				logWeek := logTime.YearDay() / 7
				tWeek := t.YearDay() / 7
				if logTime.Year() == t.Year() && logWeek == tWeek {
					count++
				}
			}
			trend = append(trend, map[string]interface{}{
				"time":  t.Format("01-02"),
				"count": count,
			})
		}
	case "month":
		for i := 11; i >= 0; i-- {
			t := now.AddDate(0, -i, 0)
			count := 0
			for _, log := range apiKeyUsage {
				logTime, err := time.Parse("2006-01-02 15:04:05", log.RequestTime)
				if err != nil {
					continue
				}
				if logTime.Year() == t.Year() && logTime.Month() == t.Month() {
					count++
				}
			}
			trend = append(trend, map[string]interface{}{
				"time":  t.Format("01-02"),
				"count": count,
			})
		}
	}

	return trend
}

// cleanAPIUsage 清理过期条目并重写 jsonl 文件。
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

	os.MkdirAll(filepath.Dir(apiUsageFile), 0755)
	f, err := os.OpenFile(apiUsageFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	for _, entry := range apiKeyUsage {
		data, _ := json.Marshal(entry)
		f.Write(data)
		f.WriteString("\n")
	}
}
