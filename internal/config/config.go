package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type Config struct {
	ListenPort           string `json:"listen_port"`
	NginxUpstreamURL     string `json:"nginx_upstream_url"`
	ClientTimeout        int    `json:"client_timeout"`
	UpstreamTimeout      int    `json:"upstream_timeout"`
	SSERecoveryEnable    bool   `json:"sse_recovery_enable"`
	DefaultRecoveryMode  string `json:"default_recovery_mode"`
	MaxRetryTimes        int    `json:"max_retry_times"`
	SessionExpireMinute  int    `json:"session_expire_minute"`
	LogKeepDays          int    `json:"log_keep_days"`
	BufferMode           bool   `json:"buffer_mode"`
}

type Prompt struct {
	ID      string `json:"id"`
	Mode    string `json:"mode"`
	Name    string `json:"name"`
	Enable  bool   `json:"enable"`
	Content string `json:"content"`
}

var (
	configInstance  *Config
	promptsInstance []Prompt
	configMutex     sync.RWMutex
)

const (
	configFile  = "./data/config.json"
	promptsFile = "./data/prompts.json"
)

func Load() (*Config, error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	if configInstance != nil {
		return configInstance, nil
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		configInstance = &Config{
			ListenPort:          "3301",
			NginxUpstreamURL:    "http://127.0.0.1:8080",
			ClientTimeout:       300,
			UpstreamTimeout:     300,
			SSERecoveryEnable:   true,
			DefaultRecoveryMode: "B",
			MaxRetryTimes:       5,
			SessionExpireMinute: 30,
			LogKeepDays:         5,
			BufferMode:          false,
		}
		if err := saveConfig(configInstance); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
		return configInstance, nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	configInstance = &Config{}
	if err := json.Unmarshal(data, configInstance); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if configInstance.ClientTimeout == 0 {
		configInstance.ClientTimeout = 300
	}
	if configInstance.UpstreamTimeout == 0 {
		configInstance.UpstreamTimeout = 300
	}

	return configInstance, nil
}

func saveConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
}

func UpdateConfig(newCfg *Config) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if err := saveConfig(newCfg); err != nil {
		return err
	}
	configInstance = newCfg
	return nil
}

func GetConfig(c *gin.Context) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	if configInstance == nil {
		c.JSON(500, gin.H{"error": "Config not loaded"})
		return
	}
	c.JSON(200, configInstance)
}

func UpdateConfigHandler(c *gin.Context) {
	var newCfg Config
	if err := c.ShouldBindJSON(&newCfg); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := UpdateConfig(&newCfg); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Config updated successfully"})
}

func loadPromptsFromFile() []Prompt {
	if _, err := os.Stat(promptsFile); os.IsNotExist(err) {
		defaultPrompts := []Prompt{
			{
				ID:      "mode_a_default",
				Mode:    "mode_a",
				Name:    "模式A-接续生成",
				Enable:  true,
				Content: "你之前已经生成以下内容：{{previous_output}}\n请继续接着已有内容输出。不要重复之前内容。保持上下文一致。",
			},
			{
				ID:      "mode_b_default",
				Mode:    "mode_b",
				Name:    "模式B-完整重生成",
				Enable:  true,
				Content: "你需要重新回答用户的问题。以下内容是之前模型生成失败的部分内容：{{previous_output}}\n请不要继续错误内容。请重新生成完整、连续、准确的答案。用户问题：{{question}}",
			},
		}
		data, _ := json.MarshalIndent(defaultPrompts, "", "  ")
		os.WriteFile(promptsFile, data, 0644)
		return defaultPrompts
	}

	data, err := os.ReadFile(promptsFile)
	if err != nil {
		return []Prompt{}
	}

	var prompts []Prompt
	if err := json.Unmarshal(data, &prompts); err != nil {
		return []Prompt{}
	}

	for i := range prompts {
		if prompts[i].Mode == "" {
			if strings.HasPrefix(prompts[i].ID, "mode_a") {
				prompts[i].Mode = "mode_a"
			} else if strings.HasPrefix(prompts[i].ID, "mode_b") {
				prompts[i].Mode = "mode_b"
			}
		}
	}

	return prompts
}

func savePromptsToFile(prompts []Prompt) error {
	data, err := json.MarshalIndent(prompts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(promptsFile, data, 0644)
}

func GetPrompts(c *gin.Context) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	prompts := loadPromptsFromFile()
	c.JSON(200, prompts)
}

func CreatePrompt(c *gin.Context) {
	var prompt Prompt
	if err := c.ShouldBindJSON(&prompt); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if prompt.Mode != "mode_a" && prompt.Mode != "mode_b" {
		c.JSON(400, gin.H{"error": "模式只能是 mode_a 或 mode_b"})
		return
	}

	configMutex.Lock()
	defer configMutex.Unlock()

	prompt.ID = fmt.Sprintf("%s_%d", prompt.Mode, time.Now().UnixNano())
	prompts := loadPromptsFromFile()
	prompts = append(prompts, prompt)

	if err := savePromptsToFile(prompts); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, prompt)
}

func UpdatePrompt(c *gin.Context) {
	id := c.Param("id")
	var updatedPrompt Prompt
	if err := c.ShouldBindJSON(&updatedPrompt); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	configMutex.Lock()
	defer configMutex.Unlock()

	prompts := loadPromptsFromFile()

	found := false
	for i, p := range prompts {
		if p.ID == id {
			updatedPrompt.ID = id
			prompts[i] = updatedPrompt
			found = true
			break
		}
	}

	if !found {
		c.JSON(404, gin.H{"error": "Prompt not found"})
		return
	}

	if err := savePromptsToFile(prompts); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, updatedPrompt)
}

func DeletePrompt(c *gin.Context) {
	id := c.Param("id")

	configMutex.Lock()
	defer configMutex.Unlock()

	prompts := loadPromptsFromFile()

	found := false
	for i, p := range prompts {
		if p.ID == id {
			prompts = append(prompts[:i], prompts[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		c.JSON(404, gin.H{"error": "Prompt not found"})
		return
	}

	if err := savePromptsToFile(prompts); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Prompt deleted"})
}

func GetPromptsByMode(mode string) []Prompt {
	configMutex.RLock()
	defer configMutex.RUnlock()

	prompts := loadPromptsFromFile()
	var result []Prompt
	for _, p := range prompts {
		if p.Mode == mode && p.Enable {
			result = append(result, p)
		}
	}
	return result
}

func GetPromptByID(id string) (*Prompt, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	prompts := loadPromptsFromFile()
	for _, p := range prompts {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("prompt not found: %s", id)
}