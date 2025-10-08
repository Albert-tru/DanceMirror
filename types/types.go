package types

import "time"

// User 用户结构
type User struct {
ID        int       `json:"id"`
Email     string    `json:"email,omitempty"`
Phone     string    `json:"phone"`
Password  string    `json:"-"` // 不返回给前端
FirstName string    `json:"firstName"`
LastName  string    `json:"lastName"`
CreatedAt time.Time `json:"createdAt"`
}

// RegisterUserPayload 用户注册请求
type RegisterUserPayload struct {
Phone     string `json:"phone" validate:"required,min=11,max=11"`
Password  string `json:"password" validate:"required,min=6,max=130"`
FirstName string `json:"firstName" validate:"required"`
LastName  string `json:"lastName" validate:"required"`
}

// LoginUserPayload 用户登录请求
type LoginUserPayload struct {
Phone    string `json:"phone" validate:"required"`
Password string `json:"password" validate:"required"`
}

// Video 视频结构
type Video struct {
ID          int       `json:"id"`
UserID      int       `json:"userId"`
Title       string    `json:"title"`
Description string    `json:"description"`
FilePath    string    `json:"filePath"`
FileName    string    `json:"fileName"`
FileSize    int64     `json:"fileSize"`
Duration    float64   `json:"duration,omitempty"` // 视频时长（秒）
Thumbnail   string    `json:"thumbnail,omitempty"` // 缩略图路径
CreatedAt   time.Time `json:"createdAt"`
UpdatedAt   time.Time `json:"updatedAt"`
}

// UploadVideoPayload 视频上传请求
type UploadVideoPayload struct {
Title       string `json:"title" validate:"required"`
Description string `json:"description"`
}

// Practice 练习记录结构
type Practice struct {
ID        int       `json:"id"`
UserID    int       `json:"userId"`
VideoID   int       `json:"videoId"`
Duration  int       `json:"duration"`  // 练习时长（秒）
Speed     float64   `json:"speed"`     // 播放速度
Notes     string    `json:"notes"`     // 练习笔记
CreatedAt time.Time `json:"createdAt"`
}

// CreatePracticePayload 创建练习记录请求
type CreatePracticePayload struct {
VideoID  int     `json:"videoId" validate:"required"`
Duration int     `json:"duration" validate:"required,min=1"`
Speed    float64 `json:"speed" validate:"required,min=0.5,max=2.0"`
Notes    string  `json:"notes"`
}

// UserStore 用户存储接口
type UserStore interface {
GetUserByEmail(email string) (*User, error)
GetUserByPhone(phone string) (*User, error)
GetUserByID(id int) (*User, error)
CreateUser(User) error
}

// VideoStore 视频存储接口
type VideoStore interface {
GetVideos(userID int) ([]*Video, error)
GetVideoByID(id int) (*Video, error)
CreateVideo(video *Video) error
UpdateVideo(video *Video) error
DeleteVideo(id int) error
}

// PracticeStore 练习记录存储接口
type PracticeStore interface {
GetPractices(userID int) ([]*Practice, error)
GetPracticeByID(id int) (*Practice, error)
CreatePractice(practice *Practice) error
DeletePractice(id int) error
}
