package storage

import (
	"time"
)

// daily_stats 表保存每日请求/token 汇总，永久保留（不受日志过期清理影响）。

func AddDailyRequest(tokens int) {
	today := time.Now().Format("2006-01-02")
	DB.Exec(`INSERT INTO daily_stats (date, request_count, token_count) VALUES (?, 1, ?)
		ON CONFLICT(date) DO UPDATE SET
		request_count = request_count + 1,
		token_count = token_count + excluded.token_count`, today, int64(tokens))
}

// GetDailyRequestCount returns per-day request counts from startDate to endDate (inclusive).
// startDate 和 endDate 格式 "2006-01-02"。
func GetDailyRequestCount(startDate, endDate string) map[string]int64 {
	result := make(map[string]int64)
	rows, err := DB.Query(`SELECT date, request_count FROM daily_stats WHERE date >= ? AND date <= ?`, startDate, endDate)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var date string
		var count int64
		if err := rows.Scan(&date, &count); err == nil {
			result[date] = count
		}
	}
	return result
}

// GetDailyTokenCount returns per-day token counts from startDate to endDate (inclusive).
func GetDailyTokenCount(startDate, endDate string) map[string]int64 {
	result := make(map[string]int64)
	rows, err := DB.Query(`SELECT date, token_count FROM daily_stats WHERE date >= ? AND date <= ?`, startDate, endDate)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var date string
		var count int64
		if err := rows.Scan(&date, &count); err == nil {
			result[date] = count
		}
	}
	return result
}
