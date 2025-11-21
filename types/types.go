package types

import (
	"time"

	"gorm.io/gorm"
)

// User 用户结构
type User struct {
	ID        int            `json:"id" gorm:"column:id"`
	Email     string         `json:"email,omitempty" gorm:"column:email;size:255"`
	Phone     string         `json:"phone" gorm:"column:phone"`
	Password  string         `json:"-" gorm:"column:password"` // 不返回给前端
	FirstName string         `json:"firstName" gorm:"column:firstName"`
	LastName  string         `json:"lastName" gorm:"column:lastName"`
	CreatedAt time.Time      `json:"createdAt" gorm:"column:createdAt"`
	UpdatedAt time.Time      `json:"updatedAt" gorm:"column:updatedAt;autoUpdateTime:false"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"column:deletedAt;index"` //软删除：
	//GORM 会自动把 DeletedAt 设置为当前时间，而不是物理删除这条记录。

	// 关联视频列表
	Videos []Video `gorm:"foreignKey:UserID" json:"videos,omitempty"` // 用户上传的视频列表
}

// RegisterUserPayload 用户注册请求
type RegisterUserPayload struct {
	Phone     string `json:"phone" validate:"required,min=11,max=11"`
	Email     string `json:"email" validate:"omitempty,email,max=255"`
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
	ID          int        `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID      int        `json:"userId" gorm:"column:userId"` // 添加 gorm tag
	Title       string     `json:"title" gorm:"not null"`
	Description string     `json:"description"`
	FilePath    string     `json:"filePath" gorm:"column:filePath;not null"` // 添加 gorm tag
	FileName    string     `json:"fileName" gorm:"column:fileName;not null"` // 添加 gorm tag
	FileSize    int64      `json:"fileSize" gorm:"column:fileSize;not null"` // 添加 gorm tag
	Duration    float64    `json:"duration,omitempty"`
	Thumbnail   string     `json:"thumbnail,omitempty"`
	CreatedAt   time.Time  `json:"createdAt" gorm:"column:createdAt;autoCreateTime"`
	UpdatedAt   time.Time  `json:"updatedAt" gorm:"column:updatedAt;autoUpdateTime"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty" gorm:"column:deletedAt;index"`
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
	Duration  int       `json:"duration"` // 练习时长（秒）
	Speed     float64   `json:"speed"`    // 播放速度
	Notes     string    `json:"notes"`    // 练习笔记
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
