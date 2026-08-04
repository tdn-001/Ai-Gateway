package storage

import (
	"ai-gateway/internal/config"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type ModelKey struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

type ModelNode struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	URL     string     `json:"url"`
	Enabled bool       `json:"enabled"`
	Keys    []ModelKey `json:"keys"`
}

var (
	modelNodesFile = "./data/model_keys.json"
	modelNodesMu   sync.RWMutex
	modelNodes     []ModelNode
	roundRobinIdx  uint64
)

func generateNodeID() string {
	return fmt.Sprintf("node-%d", time.Now().UnixNano())
}

func LoadModelNodes() {
	modelNodesMu.Lock()
	defer modelNodesMu.Unlock()

	data, err := os.ReadFile(modelNodesFile)
	if err != nil {
		return
	}

	if len(data) == 0 || string(data[0:1]) != "[" {
		return
	}

	raw := make([]json.RawMessage, 0)
	if err := json.Unmarshal(data, &raw); err != nil || len(raw) == 0 {
		return
	}

	// 探测首条记录是否为旧扁平格式（有 key 没有 url）
	var probe struct {
		Key string `json:"key"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(raw[0], &probe); err != nil {
		return
	}

	if probe.Key != "" && probe.URL == "" {
		migrateFromFlatKeys(raw, data)
		return
	}

	var nodes []ModelNode
	if err := json.Unmarshal(data, &nodes); err != nil {
		return
	}
	modelNodes = nodes
}

func migrateFromFlatKeys(_ []json.RawMessage, _ []byte) {
	cfg, _ := config.Load()
	upstreamURL := "http://127.0.0.1:8080"
	if cfg != nil && cfg.NginxUpstreamURL != "" {
		upstreamURL = cfg.NginxUpstreamURL
	}

	// 读旧格式
	data, _ := os.ReadFile(modelNodesFile)
	var oldKeys []ModelKey
	json.Unmarshal(data, &oldKeys)

	var keys []ModelKey
	for _, k := range oldKeys {
		keys = append(keys, ModelKey{
			Key:       k.Key,
			Name:      k.Name,
			Enabled:   k.Enabled,
			CreatedAt: k.CreatedAt,
		})
	}
	if len(keys) == 0 {
		keys = []ModelKey{}
	}

	nodes := []ModelNode{
		{
			ID:      generateNodeID(),
			Name:    "默认节点",
			URL:     upstreamURL,
			Enabled: true,
			Keys:    keys,
		},
	}
	modelNodes = nodes
	saveModelNodesToFile(nodes)
}

func saveModelNodesToFile(nodes []ModelNode) error {
	data, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(modelNodesFile, data, 0644)
}

func GetAllModelNodes() []ModelNode {
	modelNodesMu.RLock()
	defer modelNodesMu.RUnlock()
	return append([]ModelNode{}, modelNodes...)
}

func GetEnabledModelNode() *ModelNode {
	modelNodesMu.RLock()
	defer modelNodesMu.RUnlock()
	for i := range modelNodes {
		if modelNodes[i].Enabled {
			node := modelNodes[i]
			return &node
		}
	}
	return nil
}

// PickNodeKey 从启用节点的启用 keys 中轮询选取下一个，严格顺序递增。
// 节点无启用 key 返回空串。
func PickNodeKey() string {
	modelNodesMu.RLock()
	defer modelNodesMu.RUnlock()

	nodeIdx := getEnabledNodeIndexUnlocked()
	if nodeIdx < 0 {
		return ""
	}

	var enabledKeys []int
	for i, k := range modelNodes[nodeIdx].Keys {
		if k.Enabled {
			enabledKeys = append(enabledKeys, i)
		}
	}
	if len(enabledKeys) == 0 {
		return ""
	}

	idx := atomic.AddUint64(&roundRobinIdx, 1) - 1
	keyIdx := enabledKeys[idx%uint64(len(enabledKeys))]
	return modelNodes[nodeIdx].Keys[keyIdx].Key
}

func getEnabledNodeIndexUnlocked() int {
	for i := range modelNodes {
		if modelNodes[i].Enabled {
			return i
		}
	}
	return -1
}

func CreateModelNode(name, url string) ModelNode {
	modelNodesMu.Lock()
	defer modelNodesMu.Unlock()

	node := ModelNode{
		ID:      generateNodeID(),
		Name:    name,
		URL:     url,
		Enabled: len(modelNodes) == 0,
		Keys:    []ModelKey{},
	}
	modelNodes = append(modelNodes, node)
	saveModelNodesToFile(modelNodes)
	return node
}

func UpdateModelNode(id, name, url string, keys []ModelKey) bool {
	modelNodesMu.Lock()
	defer modelNodesMu.Unlock()

	for i := range modelNodes {
		if modelNodes[i].ID == id {
			modelNodes[i].Name = name
			modelNodes[i].URL = url
			if keys != nil {
				modelNodes[i].Keys = keys
			}
			saveModelNodesToFile(modelNodes)
			return true
		}
	}
	return false
}

func DeleteModelNode(id string) bool {
	modelNodesMu.Lock()
	defer modelNodesMu.Unlock()

	for i := range modelNodes {
		if modelNodes[i].ID == id {
			wasEnabled := modelNodes[i].Enabled
			modelNodes = append(modelNodes[:i], modelNodes[i+1:]...)

			// 删除的是唯一启用节点时，自动启用第一个剩余节点，保证仍有可用节点
			if wasEnabled && len(modelNodes) > 0 {
				modelNodes[0].Enabled = true
			}

			saveModelNodesToFile(modelNodes)
			return true
		}
	}
	return false
}

func ToggleModelNode(id string, enabled bool) bool {
	modelNodesMu.Lock()
	defer modelNodesMu.Unlock()

	// 统计当前启用节点数
	enabledCount := 0
	for j := range modelNodes {
		if modelNodes[j].Enabled {
			enabledCount++
		}
	}

	for i := range modelNodes {
		if modelNodes[i].ID == id {
			// 不允许关闭唯一的启用节点
			if !enabled && modelNodes[i].Enabled && enabledCount <= 1 {
				return false
			}
			if enabled {
				for j := range modelNodes {
					modelNodes[j].Enabled = false
				}
			}
			modelNodes[i].Enabled = enabled
			saveModelNodesToFile(modelNodes)
			return true
		}
	}
	return false
}

func CreateModelKey(nodeID, name, key string) bool {
	modelNodesMu.Lock()
	defer modelNodesMu.Unlock()

	for i := range modelNodes {
		if modelNodes[i].ID == nodeID {
			mk := ModelKey{
				Key:       key,
				Name:      name,
				Enabled:   true,
				CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
			}
			modelNodes[i].Keys = append(modelNodes[i].Keys, mk)
			saveModelNodesToFile(modelNodes)
			return true
		}
	}
	return false
}

func DeleteModelKey(nodeID, keyID string) bool {
	modelNodesMu.Lock()
	defer modelNodesMu.Unlock()

	for i := range modelNodes {
		if modelNodes[i].ID == nodeID {
			for j, k := range modelNodes[i].Keys {
				if k.Key == keyID {
					modelNodes[i].Keys = append(modelNodes[i].Keys[:j], modelNodes[i].Keys[j+1:]...)
					saveModelNodesToFile(modelNodes)
					return true
				}
			}
			return false
		}
	}
	return false
}

func ToggleModelKey(nodeID, keyID string, enabled bool) bool {
	modelNodesMu.Lock()
	defer modelNodesMu.Unlock()

	for i := range modelNodes {
		if modelNodes[i].ID == nodeID {
			for j, k := range modelNodes[i].Keys {
				if k.Key == keyID {
					modelNodes[i].Keys[j].Enabled = enabled
					saveModelNodesToFile(modelNodes)
					return true
				}
			}
			return false
		}
	}
	return false
}
