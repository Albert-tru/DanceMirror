package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/types"
	"github.com/Albert-tru/DanceMirror/utils"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserKey contextKey = "userID"

func CreateJWT(secret []byte, userID int64) (string, error) {
	expiration, err := time.ParseDuration(config.Envs.JWTExpiration)
	if err != nil {
		expiration = time.Hour * 72
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userID":    strconv.FormatInt(userID, 10),
		"expiresAt": time.Now().Add(expiration).Unix(),
	})

	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func WithJWTAuth(handlerFunc http.HandlerFunc, store types.UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenString := utils.GetTokenFromRequest(r)
		token, err := validateJWT(tokenString)
		if err != nil || !token.Valid {
			if err != nil {
				log.Printf("failed to validate token: %v", err)
			}
			permissionDenied(w)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			permissionDenied(w)
			return
		}

		str, ok := claims["userID"].(string)
		if !ok {
			permissionDenied(w)
			return
		}

		userID, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			log.Printf("failed to convert userID to int64: %v", err)
			permissionDenied(w)
			return
		}

		u, err := store.GetUserByID(userID)
		if err != nil {
			log.Printf("failed to get user by id: %v", err)
			permissionDenied(w)
			return
		}

		if strings.EqualFold(u.AccountStatus, "disabled") || strings.EqualFold(u.AccountStatus, "banned") {
			permissionDenied(w)
			return
		}

		ctx := context.WithValue(r.Context(), UserKey, u.ID)
		handlerFunc(w, r.WithContext(ctx))
	}
}

func WithAdminAuth(handlerFunc http.HandlerFunc, store types.UserStore) http.HandlerFunc {
	return WithJWTAuth(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserIDFromContext(r.Context())
		u, err := store.GetUserByID(userID)
		if err != nil {
			permissionDenied(w)
			return
		}
		if !strings.EqualFold(u.Role, "admin") {
			permissionDenied(w)
			return
		}
		handlerFunc(w, r)
	}, store)
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

func GetUserIDFromRequest(r *http.Request) int64 {
	tokenString := utils.GetTokenFromRequest(r)
	if tokenString == "" {
		return -1
	}

	token, err := validateJWT(tokenString)
	if err != nil || token == nil || !token.Valid {
		return -1
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return -1
	}

	rawUserID, ok := claims["userID"]
	if !ok {
		return -1
	}

	switch v := rawUserID.(type) {
	case string:
		userID, parseErr := strconv.ParseInt(v, 10, 64)
		if parseErr != nil {
			return -1
		}
		return userID
	case float64:
		return int64(v)
	default:
		return -1
	}
}
