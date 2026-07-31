package main

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

// frontendNavigationMiddleware 拦截浏览器直接导航（刷新/地址栏输入）到与后端 API
// 路径冲突的前端路由（如 /admin/apikeys、/admin/logs），返回 SPA index.html 而非 401。
// 判断依据：GET 请求 + Accept 头包含 text/html（浏览器导航），且未携带 Authorization。
func frontendNavigationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			accept := c.Request.Header.Get("Accept")
			auth := c.Request.Header.Get("Authorization")
			if strings.Contains(accept, "text/html") && auth == "" {
				indexData, err := frontendFS.ReadFile("frontend/dist/index.html")
				if err == nil {
					c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
					c.Abort()
					return
				}
				indexPath := filepath.Join("./frontend/dist", "index.html")
				if _, statErr := os.Stat(indexPath); statErr == nil {
					c.File(indexPath)
					c.Abort()
					return
				}
			}
		}
		c.Next()
	}
}

func setupFrontend(r *gin.Engine) {
	subFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err == nil {
		if _, err := subFS.Open("index.html"); err == nil {
			fileServer := http.FileServer(http.FS(subFS))

			r.GET("/assets/*filepath", func(c *gin.Context) {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
				gin.WrapH(fileServer)(c)
			})
			r.GET("/favicon.svg", gin.WrapH(fileServer))
			r.GET("/icons.svg", gin.WrapH(fileServer))
			r.GET("/logo.png", gin.WrapH(fileServer))

			r.NoRoute(func(c *gin.Context) {
				if strings.HasPrefix(c.Request.URL.Path, "/v1") ||
					strings.HasPrefix(c.Request.URL.Path, "/api") ||
					c.Request.URL.Path == "/health" {
					c.JSON(404, gin.H{"error": "Not found"})
					return
				}
				indexData, _ := frontendFS.ReadFile("frontend/dist/index.html")
				c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
			})
			return
		}
	}

	frontendDir := "./frontend/dist"
	r.Static("/assets", filepath.Join(frontendDir, "assets"))
	r.StaticFile("/favicon.ico", filepath.Join(frontendDir, "favicon.ico"))
	r.StaticFile("/logo.png", filepath.Join(frontendDir, "logo.png"))
	r.NoRoute(func(c *gin.Context) {
		indexPath := filepath.Join(frontendDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			c.File(indexPath)
		} else {
			c.JSON(404, gin.H{"error": "Not found"})
		}
	})
}

func init() {
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".css", "text/css")
	mime.AddExtensionType(".svg", "image/svg+xml")
	mime.AddExtensionType(".woff2", "font/woff2")
	mime.AddExtensionType(".wasm", "application/wasm")
}
