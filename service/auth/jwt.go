package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/types"
	"github.com/Albert-tru/DanceMirror/utils"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserKey contextKey = "userID"

//三步走：
//1. CreateJWT：生成 JWT，包含用户 ID 和过期时间等信息，并使用密钥进行签名。
//2. WithJWTAuth：一个中间件函数，验证请求中的 JWT 是否有效，并将用户 ID 存储在请求上下文中，以供后续处理使用。
//3. GetUserIDFromContext：从请求上下文中获取用户 ID 的函数，供需要用户信息的处理函数调用。

func CreateJWT(secret []byte, userID int64) (string, error) {
	//解析 JWT 过期时间配置，如果解析失败，默认设置为 72 小时
	expiration, err := time.ParseDuration(config.Envs.JWTExpiration)
	if err != nil {
		expiration = time.Hour * 72
	}

	//生成JWT，包含用户ID和过期时间等信息
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userID":    strconv.FormatInt(userID, 10),
		"expiresAt": time.Now().Add(expiration).Unix(),
	})

	//使用密钥进行签名，生成 JWT 字符串
	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func WithJWTAuth(handlerFunc http.HandlerFunc, store types.UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//1、提取token：从请求中提取 JWT，通常是从 Authorization 头部获取。
		tokenString := utils.GetTokenFromRequest(r)

		//2、校验token：验证 JWT 的有效性，包括签名和过期时间等信息。
		token, err := validateJWT(tokenString)
		if err != nil {
			log.Printf("failed to validate token: %v", err)
			permissionDenied(w)
			return
		}
		if !token.Valid {
			log.Println("invalid token")
			permissionDenied(w)
			return
		}

		//3、从 Token 的载荷中拿出 userID 字符串，解析回 int64
		claims := token.Claims.(jwt.MapClaims)
		str := claims["userID"].(string)

		userID, err := strconv.ParseInt(str, 10, 64) // 改为 ParseInt 获取 int64
		if err != nil {
			log.Printf("failed to convert userID to int: %v", err)
			permissionDenied(w)
			return
		}

		//4、数据库二次校验
		u, err := store.GetUserByID(userID)
		if err != nil {
			log.Printf("failed to get user by id: %v", err)
			permissionDenied(w)
			return
		}

		//5、上细纹注入，将用户 ID 存储在请求上下文中，以供后续处理使用。
		ctx := r.Context()
		ctx = context.WithValue(ctx, UserKey, u.ID)
		r = r.WithContext(ctx)

		handlerFunc(w, r)
	}
}

func validateJWT(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.Envs.JWTSecret), nil
	})
}

func permissionDenied(w http.ResponseWriter) {
	utils.WriteError(w, http.StatusForbidden, fmt.Errorf("permission denied"))
}

func GetUserIDFromContext(ctx context.Context) int64 {
	userID, ok := ctx.Value(UserKey).(int64)
	if !ok {
		return -1
	}
	return userID
}
