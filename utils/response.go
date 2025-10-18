package utils

import (
	"net/http"
)

// Response 统一的 API 响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// 业务状态码定义
const (
	CodeSuccess         = 200
	CodeBadRequest      = 400
	CodeUnauthorized    = 401
	CodeForbidden       = 403
	CodeNotFound        = 404
	CodeConflict        = 409
	CodeValidationError = 422
	CodeTooManyRequests = 429
	CodeInternalError   = 500
)

// Success 成功响应
func Success(w http.ResponseWriter, data interface{}, message string) {
	if message == "" {
		message = "Success"
	}
	WriteJSON(w, http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: message,
		Data:    data,
	})
}

// Error 错误响应
func Error(w http.ResponseWriter, code int, message string, err error) {
	var errorDetail string
	if err != nil {
		errorDetail = err.Error()
	}

	httpStatus := code
	if code >= 500 {
		httpStatus = http.StatusInternalServerError
	} else if code >= 400 && code < 500 {
		httpStatus = code
	} else {
		httpStatus = http.StatusBadRequest
	}

	WriteJSON(w, httpStatus, Response{
		Code:    code,
		Message: message,
		Error:   errorDetail,
	})
}

// BadRequest 400 错误
func BadRequest(w http.ResponseWriter, message string, err error) {
	if message == "" {
		message = "请求参数错误"
	}
	Error(w, CodeBadRequest, message, err)
}

// Unauthorized 401 错误
func Unauthorized(w http.ResponseWriter, message string) {
	if message == "" {
		message = "未授权访问"
	}
	Error(w, CodeUnauthorized, message, nil)
}

// Forbidden 403 错误
func Forbidden(w http.ResponseWriter, message string) {
	if message == "" {
		message = "禁止访问"
	}
	Error(w, CodeForbidden, message, nil)
}

// NotFound 404 错误
func NotFound(w http.ResponseWriter, message string) {
	if message == "" {
		message = "资源不存在"
	}
	Error(w, CodeNotFound, message, nil)
}

// Conflict 409 错误
func Conflict(w http.ResponseWriter, message string) {
	if message == "" {
		message = "资源冲突"
	}
	Error(w, CodeConflict, message, nil)
}

// ValidationError 422 参数验证错误
func ValidationError(w http.ResponseWriter, message string, err error) {
	if message == "" {
		message = "参数验证失败"
	}
	Error(w, CodeValidationError, message, err)
}

// TooManyRequests 429 请求过于频繁
func TooManyRequests(w http.ResponseWriter, message string) {
	if message == "" {
		message = "请求过于频繁，请稍后再试"
	}
	Error(w, CodeTooManyRequests, message, nil)
}

// InternalError 500 服务器内部错误
func InternalError(w http.ResponseWriter, message string, err error) {
	if message == "" {
		message = "服务器内部错误"
	}
	Error(w, CodeInternalError, message, err)
}
