package user

import (
	"database/sql"
	"fmt"

	"github.com/Albert-tru/DanceMirror/types"
	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetUserByEmail(email string) (*types.User, error) {
	var user types.User
	result := s.db.Where("email = ?", email).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func (s *Store) GetUserByPhone(phone string) (*types.User, error) {
	var user types.User
	result := s.db.Where("phone = ?", phone).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func (s *Store) GetUserByID(id int) (*types.User, error) {
	var user types.User
	result := s.db.First(&user, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func (s *Store) CreateUser(user types.User) error {
	result := s.db.Create(&user)
	if result.Error != nil {
		return fmt.Errorf("failed to create user: %w", result.Error)
	}
	return nil
}

func (s *Store) UpdateUser(user *types.User) error {
	return s.db.Save(user).Error
}

func (s *Store) DeleteUser(id uint) error {
	return s.db.Delete(&types.User{}, id).Error // 软删除
}

func scanRowIntoUser(rows *sql.Rows) (*types.User, error) {
	user := new(types.User)

	err := rows.Scan(
		&user.ID,
		&user.Email,
		&user.Phone,
		&user.Password,
		&user.FirstName,
		&user.LastName,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}
