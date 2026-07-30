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
				if strings.HasPrefix(c.Request.URL.Path, "/admin") ||
					strings.HasPrefix(c.Request.URL.Path, "/v1") ||
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
