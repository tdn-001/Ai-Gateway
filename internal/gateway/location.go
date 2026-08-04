package gateway

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"
)

type IPLocation struct {
	IP        string  `json:"ip"`
	Country   string  `json:"country"`
	Region    string  `json:"regionName"`
	City      string  `json:"city"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Status    string  `json:"status"`
	Query     string  `json:"query"`
}

var (
	ipCache   = make(map[string]*IPLocation)
	ipCacheMu sync.RWMutex
)

func isPrivateIP(ip string) bool {
	if ip == "127.0.0.1" || ip == "localhost" || ip == "::1" {
		return true
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if parsed.IsLoopback() || parsed.IsPrivate() {
		return true
	}
	return false
}

// GetIPLocation 有 IP 请求归属地时动态查询一次，结果缓存到内存，
// 同一进程内同一 IP 后续直接命中缓存，不再重复查询。
func GetIPLocation(ip string) *IPLocation {
	if isPrivateIP(ip) {
		return &IPLocation{
			IP:      ip,
			Country: "本地",
			Region:  "本地",
			City:    "本地",
			Status:  "success",
		}
	}

	ipCacheMu.RLock()
	if cached, exists := ipCache[ip]; exists {
		ipCacheMu.RUnlock()
		return cached
	}
	ipCacheMu.RUnlock()

	cacheFail := func() *IPLocation {
		loc := &IPLocation{IP: ip, Status: "fail"}
		ipCacheMu.Lock()
		ipCache[ip] = loc
		ipCacheMu.Unlock()
		return loc
	}

	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get("http://ip-api.com/json/" + ip + "?lang=zh-CN")
	if err != nil {
		return cacheFail()
	}
	defer resp.Body.Close()

	var location IPLocation
	if err := json.NewDecoder(resp.Body).Decode(&location); err != nil {
		return cacheFail()
	}

	if location.Status != "success" {
		return cacheFail()
	}

	ipCacheMu.Lock()
	ipCache[ip] = &location
	ipCacheMu.Unlock()

	return &location
}