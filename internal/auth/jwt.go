package auth

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Admin struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	CreatedAt    string `json:"created_at"`
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

var (
	jwtSecret     = getJWTSecret()
	adminInstance *Admin
	adminMutex    sync.RWMutex
)

const adminFile = "./data/admin.json"

func getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "ai-gateway-secret-key-2024"
	}
	return []byte(secret)
}

func Init() {
	adminMutex.Lock()
	defer adminMutex.Unlock()

	if _, err := os.Stat(adminFile); os.IsNotExist(err) {
		adminInstance = nil
		return
	}

	data, err := os.ReadFile(adminFile)
	if err != nil {
		return
	}

	adminInstance = &Admin{}
	json.Unmarshal(data, adminInstance)
}

func saveAdmin(admin *Admin) error {
	data, err := json.MarshalIndent(admin, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(adminFile, data, 0644)
}

func IsAdminExists() bool {
	adminMutex.RLock()
	defer adminMutex.RUnlock()
	return adminInstance != nil
}

func Register(c *gin.Context) {
	adminMutex.Lock()
	defer adminMutex.Unlock()

	if adminInstance != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "管理员账号已存在，不能重复注册"})
		return
	}

	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Username) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名至少3个字符"})
		return
	}

	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码至少6个字符"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	adminInstance = &Admin{
		Username:     req.Username,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}

	if err := saveAdmin(adminInstance); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存管理员失败"})
		return
	}

	claims := &Claims{
		Username: req.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成Token失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString, "message": "注册成功"})
}

func Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminMutex.RLock()
	defer adminMutex.RUnlock()

	if adminInstance == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未注册管理员账号，请先注册"})
		return
	}

	if req.Username != adminInstance.Username {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(adminInstance.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	claims := &Claims{
		Username: req.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成Token失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")

		if tokenString == "" || tokenString == "Bearer " {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "需要登录认证"})
			c.Abort()
			return
		}

		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token无效或已过期"})
			c.Abort()
			return
		}

		c.Set("username", claims.Username)
		c.Next()
	}
}

func ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminMutex.Lock()
	defer adminMutex.Unlock()

	if adminInstance == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "管理员未初始化"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(adminInstance.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "原密码错误"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	adminInstance.PasswordHash = string(hash)
	if err := saveAdmin(adminInstance); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}