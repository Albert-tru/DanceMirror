package utils

import (
"fmt"
"github.com/go-playground/validator/v10"
)

// ValidateStruct 验证结构体
func ValidateStruct(s interface{}) error {
return Validate.Struct(s)
}

// FormatValidationError 格式化验证错误为中文提示
func FormatValidationError(err error) string {
if err == nil {
return ""
}

validationErrors, ok := err.(validator.ValidationErrors)
if !ok {
return err.Error()
}

for _, e := range validationErrors {
field := e.Field()
tag := e.Tag()

switch tag {
case "required":
return fmt.Sprintf("%s 不能为空", field)
case "email":
return fmt.Sprintf("%s 邮箱格式不正确", field)
case "min":
return fmt.Sprintf("%s 长度不能小于 %s", field, e.Param())
case "max":
return fmt.Sprintf("%s 长度不能大于 %s", field, e.Param())
case "len":
return fmt.Sprintf("%s 长度必须为 %s", field, e.Param())
case "numeric":
return fmt.Sprintf("%s 必须为数字", field)
case "alphanum":
return fmt.Sprintf("%s 只能包含字母和数字", field)
default:
return fmt.Sprintf("%s 验证失败: %s", field, tag)
}
}

return "参数验证失败"
}

// ValidatePhone 验证手机号（中国大陆）
func ValidatePhone(phone string) bool {
if len(phone) != 11 {
return false
}

// 简单验证：以1开头的11位数字
if phone[0] != '1' {
return false
}

for _, c := range phone {
if c < '0' || c > '9' {
return false
}
}

return true
}

// ValidatePassword 验证密码强度
func ValidatePassword(password string) (bool, string) {
if len(password) < 6 {
return false, "密码长度至少6位"
}

if len(password) > 20 {
return false, "密码长度不能超过20位"
}

return true, ""
}
