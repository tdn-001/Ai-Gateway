package storage

import (
	"database/sql"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

const dbFile = "./db/gateway.db"

var DB *sql.DB

func InitDB() error {
	if err := os.MkdirAll("./db", 0755); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", dbFile+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return err
	}

	DB = db
	return createSchema()
}

func createSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS token_stats (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			total_tokens INTEGER NOT NULL DEFAULT 0,
			today_tokens INTEGER NOT NULL DEFAULT 0,
			last_reset_date TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS daily_stats (
			date TEXT PRIMARY KEY,
			request_count INTEGER NOT NULL DEFAULT 0,
			token_count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS api_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL,
			request_id TEXT NOT NULL,
			client_ip TEXT NOT NULL,
			request_time TEXT NOT NULL,
			cost REAL NOT NULL,
			status INTEGER NOT NULL,
			model TEXT NOT NULL,
			prompt_tokens INTEGER DEFAULT 0,
			completion_tokens INTEGER DEFAULT 0,
			total_tokens INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_usage_time ON api_usage(request_time)`,
		`CREATE INDEX IF NOT EXISTS idx_api_usage_key ON api_usage(key)`,
		`CREATE TABLE IF NOT EXISTS request_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL,
			client_ip TEXT NOT NULL,
			request_time TEXT NOT NULL,
			cost REAL NOT NULL,
			status INTEGER NOT NULL,
			status_chain TEXT,
			request_body TEXT,
			recover INTEGER DEFAULT 0,
			recover_count INTEGER DEFAULT 0,
			error TEXT,
			error_phase TEXT,
			retry_count INTEGER DEFAULT 0,
			partial_output TEXT,
			result TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_time ON request_logs(request_time)`,
		`CREATE TABLE IF NOT EXISTS upstream_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL,
			client_ip TEXT NOT NULL,
			request_time TEXT NOT NULL,
			cost REAL NOT NULL,
			status INTEGER NOT NULL,
			model TEXT,
			stream INTEGER DEFAULT 0,
			error TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_upstream_logs_time ON upstream_logs(request_time)`,
	}

	for _, s := range stmts {
		if _, err := DB.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

func initTokenStatsRow() {
	var id int
	err := DB.QueryRow(`SELECT id FROM token_stats WHERE id = 1`).Scan(&id)
	if err == sql.ErrNoRows {
		DB.Exec(`INSERT INTO token_stats (id, total_tokens, today_tokens, last_reset_date) VALUES (1, 0, 0, ?)`, time.Now().Format("2006-01-02"))
	}
}
