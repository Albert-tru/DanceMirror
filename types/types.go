package types

import "time"

type User struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	CreatedAt time.Time `json:"createdAt"`
}

type RegisterUserPayload struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=6,max=130"`
	FirstName string `json:"firstName" validate:"required"`
	LastName  string `json:"lastName" validate:"required"`
}

type LoginUserPayload struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type Video struct {
	ID          int       `json:"id"`
	UserID      int       `json:"userId"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	FilePath    string    `json:"filePath"`
	FileName    string    `json:"fileName"`
	FileSize    int64     `json:"fileSize"`
	Duration    float64   `json:"duration,omitempty"`
	Thumbnail   string    `json:"thumbnail,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type UserStore interface {
	GetUserByEmail(email string) (*User, error)
	GetUserByID(id int) (*User, error)
	CreateUser(User) error
}

type VideoStore interface {
	GetVideoByID(id int) (*Video, error)
	GetVideosByUserID(userID int) ([]*Video, error)
	CreateVideo(video *Video) error
	UpdateVideo(video *Video) error
	DeleteVideo(id int) error
}
