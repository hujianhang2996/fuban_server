package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var secretKey = do.jwt_secret()

// const jwtExpTime = 60 * 24 * time.Hour
// const jwtRefeshTime = 45 * 24 * time.Hour
const jwtExpTime = 30 * 24 * time.Hour
const jwtRefeshTime = 25 * 24 * time.Hour

type User struct {
	Username             string `json:"username"`
	IP                   string `json:"ip"`
	jwt.RegisteredClaims        // v5版本新加的方法
}

func GenerateJWT(username, ip string) (string, error) {
	claims := User{
		username,
		ip,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtExpTime)), // 过期时间
			IssuedAt:  jwt.NewNumericDate(time.Now()),                 // 签发时间
			NotBefore: jwt.NewNumericDate(time.Now()),                 // 生效时间
		},
	}
	// 使用HS256签名算法
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := t.SignedString([]byte(secretKey))

	return s, err
}

func ParseJwt(tokenstring, ip string) (*User, error) {
	t, err := jwt.ParseWithClaims(tokenstring, &User{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := t.Claims.(*User); ok && t.Valid {
		if claims.Username == Username || claims.IP == ip {
			return claims, nil
		} else {
			return nil, errors.New("invalid claims")
		}
	} else {
		return nil, err
	}
}

func jwtParseMiddleWare(c *gin.Context) {
	var token string
	if c.Request.URL.Path == "/api/ws" {
		token = c.Query("token")
	} else {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusOK, gin.H{"type": "error", "message": "Authorization header is missing"})
			c.Abort()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusOK, gin.H{"type": "error", "message": "Invalid Authorization format"})
			c.Abort()
			return
		}
		token = parts[1]
	}
	_, err := ParseJwt(token, strings.Split(c.Request.RemoteAddr, ":")[0])
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"type": "error", "message": err.Error()})
		c.Abort()
		return
	}
	c.Next()
}

func jwtAuth(c *gin.Context) {
	authHeader := c.Request.Header.Get("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusOK, gin.H{"type": "error", "message": "Authorization header is missing"})
		return
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusOK, gin.H{"type": "error", "message": "Invalid Authorization format"})
		return
	}
	token := parts[1]
	claims, err := ParseJwt(token, strings.Split(c.Request.RemoteAddr, ":")[0])
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"type": "error", "message": err.Error()})
		return
	}
	if time.Until(claims.ExpiresAt.Time) < jwtRefeshTime {
		token, err := GenerateJWT(claims.Username, claims.IP)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"type": "success", "message": "login succeed but refresh token failed"})
		} else {
			c.JSON(http.StatusOK, gin.H{"type": "refresh", "message": "login and refresh succeed", "token": token})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"type": "success", "message": ""})
}

func login(c *gin.Context) {
	input := gin.H{"username": "", "password": ""}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	encoded_username := input["username"].(string)
	encoded_password := input["password"].(string)
	var info StaticInfo
	do.db.First(&info, "info_name = ?", "username")
	username := info.InfoValue
	do.db.First(&info, "info_name = ?", "password")
	password := info.InfoValue
	decoded_username := rsaDecode(encoded_username)
	decoded_password := rsaDecode(encoded_password)
	match, err := argon2id.ComparePasswordAndHash(decoded_password, password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "message": err.Error()})
		return
	}
	if decoded_username == username && match {
		token, err := GenerateJWT(username, strings.Split(c.Request.RemoteAddr, ":")[0])
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"type": "error", "message": "JWT encode error: " + err.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"type": "success", "message": "login succeed", "token": token})
		}
	} else {
		c.JSON(http.StatusOK, gin.H{"type": "error", "message": "wrong username or password"})
	}
}

func test2() {
	s, err := GenerateJWT("zhangsan", "127.0.0.1")
	if err != nil {
		fmt.Println("generate jwt failed, ", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", s)

	// 解析jwt
	claims, err := ParseJwt(s, "127.0.0.1")
	if err != nil {
		fmt.Println("parse jwt failed, ", err)
		os.Exit(1)
	}
	fmt.Printf("%+v\n", claims)
}
