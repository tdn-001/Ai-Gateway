package storage

import (
	"database/sql"
	"time"
)

// token_stats 表保存累计与当日 token 统计，永久保留（不受日志过期清理影响）。

// LoadTokenStats 确保 token_stats 表有初始行，并处理跨天重置。
func LoadTokenStats() {
	if DB == nil {
		return
	}
	initTokenStatsRow()
	resetTokenStatsIfNewDay()
}

// resetTokenStatsIfNewDay 若上次重置日期不是今天，清零今日 token。
func resetTokenStatsIfNewDay() {
	today := time.Now().Format("2006-01-02")

	var lastReset string
	err := DB.QueryRow(`SELECT last_reset_date FROM token_stats WHERE id = 1`).Scan(&lastReset)
	if err == sql.ErrNoRows {
		initTokenStatsRow()
		return
	}
	if err != nil {
		return
	}

	if lastReset != today {
		DB.Exec(`UPDATE token_stats SET today_tokens = 0, last_reset_date = ? WHERE id = 1`, today)
	}
}

func AddTokens(promptTokens, completionTokens int) {
	total := int64(promptTokens) + int64(completionTokens)
	if total <= 0 {
		return
	}

	today := time.Now().Format("2006-01-02")

	var lastReset string
	err := DB.QueryRow(`SELECT last_reset_date FROM token_stats WHERE id = 1`).Scan(&lastReset)
	if err == sql.ErrNoRows {
		initTokenStatsRow()
	} else if err == nil && lastReset != today {
		DB.Exec(`UPDATE token_stats SET today_tokens = 0, last_reset_date = ? WHERE id = 1`, today)
	}

	DB.Exec(`UPDATE token_stats SET total_tokens = total_tokens + ?, today_tokens = today_tokens + ? WHERE id = 1`, total, total)
}

func GetTokenStats() (total int64, today int64) {
	if DB == nil {
		return 0, 0
	}

	var id int
	err := DB.QueryRow(`SELECT id FROM token_stats WHERE id = 1`).Scan(&id)
	if err == sql.ErrNoRows {
		initTokenStatsRow()
	}

	resetTokenStatsIfNewDay()

	DB.QueryRow(`SELECT total_tokens, today_tokens FROM token_stats WHERE id = 1`).Scan(&total, &today)
	return total, today
}
