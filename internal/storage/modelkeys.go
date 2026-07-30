package storage

import (
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type ModelAPIKey struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

var (
	modelKeysFile = "./data/model_keys.json"
	modelKeysMu   sync.RWMutex
	roundRobinIdx uint64
)

func loadModelKeysFromFile() []ModelAPIKey {
	if _, err := os.Stat(modelKeysFile); os.IsNotExist(err) {
		return []ModelAPIKey{}
	}

	data, err := os.ReadFile(modelKeysFile)
	if err != nil {
		return []ModelAPIKey{}
	}

	var keys []ModelAPIKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return []ModelAPIKey{}
	}
	return keys
}

func saveModelKeysToFile(keys []ModelAPIKey) error {
	data, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(modelKeysFile, data, 0644)
}

func GetAllModelKeys() []ModelAPIKey {
	modelKeysMu.RLock()
	defer modelKeysMu.RUnlock()
	return loadModelKeysFromFile()
}

func GetEnabledModelKeys() []ModelAPIKey {
	modelKeysMu.RLock()
	defer modelKeysMu.RUnlock()

	keys := loadModelKeysFromFile()
	var enabled []ModelAPIKey
	for _, k := range keys {
		if k.Enabled {
			enabled = append(enabled, k)
		}
	}
	return enabled
}

func GetEnabledModelKey() *ModelAPIKey {
	enabled := GetEnabledModelKeys()
	if len(enabled) == 0 {
		return nil
	}
	idx := atomic.AddUint64(&roundRobinIdx, 1)
	return &enabled[(idx-1)%uint64(len(enabled))]
}

func CreateModelKey(name string, key string) ModelAPIKey {
	modelKeysMu.Lock()
	defer modelKeysMu.Unlock()

	keys := loadModelKeysFromFile()
	newKey := ModelAPIKey{
		Key:       key,
		Name:      name,
		Enabled:   true,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	keys = append(keys, newKey)
	saveModelKeysToFile(keys)
	return newKey
}

func DeleteModelKey(key string) bool {
	modelKeysMu.Lock()
	defer modelKeysMu.Unlock()

	keys := loadModelKeysFromFile()
	for i, k := range keys {
		if k.Key == key {
			keys = append(keys[:i], keys[i+1:]...)
			saveModelKeysToFile(keys)
			return true
		}
	}
	return false
}

func ToggleModelKey(key string, enabled bool) bool {
	modelKeysMu.Lock()
	defer modelKeysMu.Unlock()

	keys := loadModelKeysFromFile()
	for i, k := range keys {
		if k.Key == key {
			keys[i].Enabled = enabled
			saveModelKeysToFile(keys)
			return true
		}
	}
	return false
}
