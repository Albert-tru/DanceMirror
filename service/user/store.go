package user

import (
"database/sql"
"fmt"

"github.com/Albert-tru/DanceMirror/types"
)

type Store struct {
db *sql.DB
}

func NewStore(db *sql.DB) *Store {
return &Store{db: db}
}

func (s *Store) GetUserByEmail(email string) (*types.User, error) {
rows, err := s.db.Query("SELECT * FROM users WHERE email = ?", email)
if err != nil {
return nil, err
}
defer rows.Close()

u := new(types.User)
for rows.Next() {
u, err = scanRowIntoUser(rows)
if err != nil {
return nil, err
}
}

if u.ID == 0 {
return nil, fmt.Errorf("user not found")
}

return u, nil
}

func (s *Store) GetUserByPhone(phone string) (*types.User, error) {
rows, err := s.db.Query("SELECT * FROM users WHERE phone = ?", phone)
if err != nil {
return nil, err
}
defer rows.Close()

u := new(types.User)
for rows.Next() {
u, err = scanRowIntoUser(rows)
if err != nil {
return nil, err
}
}

if u.ID == 0 {
return nil, fmt.Errorf("user not found")
}

return u, nil
}

func (s *Store) GetUserByID(id int) (*types.User, error) {
rows, err := s.db.Query("SELECT * FROM users WHERE id = ?", id)
if err != nil {
return nil, err
}
defer rows.Close()

u := new(types.User)
for rows.Next() {
u, err = scanRowIntoUser(rows)
if err != nil {
return nil, err
}
}

if u.ID == 0 {
return nil, fmt.Errorf("user not found")
}

return u, nil
}

func (s *Store) CreateUser(user types.User) error {
_, err := s.db.Exec("INSERT INTO users (email, phone, password, firstName, lastName) VALUES (?, ?, ?, ?, ?)",
user.Email, user.Phone, user.Password, user.FirstName, user.LastName)
if err != nil {
return err
}

return nil
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
