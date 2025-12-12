package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type VerifyRequest struct {
	Code string `json:"code"`
}

var tokenSecret = "stock-forecast-secret-key"

func init() {
	// 从环境变量读取密钥
	if secret := os.Getenv("TOKEN_SECRET"); secret != "" {
		tokenSecret = secret
	}
}

// generateToken 生成token: timestamp.signature
func generateToken() string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	h := hmac.New(sha256.New, []byte(tokenSecret))
	h.Write([]byte(timestamp))
	signature := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("%s.%s", timestamp, signature)
}

// ValidateToken 验证token
func ValidateToken(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}

	timestamp, signature := parts[0], parts[1]

	// 验证签名
	h := hmac.New(sha256.New, []byte(tokenSecret))
	h.Write([]byte(timestamp))
	expectedSig := hex.EncodeToString(h.Sum(nil))
	if signature != expectedSig {
		return false
	}

	// 验证是否过期（7天有效期）
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-ts > 7*24*3600 {
		return false
	}

	return true
}

// VerifyInviteCode 验证邀请码
func VerifyInviteCode(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
		})
		return
	}

	inviteCode := os.Getenv("INVITE_CODE")
	if inviteCode == "" {
		// 如果没有配置邀请码，直接通过
		token := generateToken()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "验证成功",
			"token":   token,
		})
		return
	}

	if req.Code == inviteCode {
		token := generateToken()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "验证成功",
			"token":   token,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "邀请码错误",
		})
	}
}

// AuthMiddleware 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果没有配置邀请码，跳过验证
		if os.Getenv("INVITE_CODE") == "" {
			c.Next()
			return
		}

		// 从 Header 获取 token
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "未授权访问",
			})
			c.Abort()
			return
		}

		// 去掉 Bearer 前缀
		token = strings.TrimPrefix(token, "Bearer ")

		if !ValidateToken(token) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "token无效或已过期",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
