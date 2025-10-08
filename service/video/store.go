package video

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

func (s *Store) GetVideoByID(id int) (*types.Video, error) {
	rows, err := s.db.Query("SELECT * FROM videos WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	v := new(types.Video)
	for rows.Next() {
		v, err = scanRowIntoVideo(rows)
		if err != nil {
			return nil, err
		}
	}

	if v.ID == 0 {
		return nil, fmt.Errorf("video not found")
	}

	return v, nil
}

func (s *Store) GetVideos(userID int) ([]*types.Video, error) {
	rows, err := s.db.Query("SELECT * FROM videos WHERE userId = ? ORDER BY createdAt DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	videos := []*types.Video{}
	for rows.Next() {
		v, err := scanRowIntoVideo(rows)
		if err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}

	return videos, nil
}

func (s *Store) CreateVideo(video *types.Video) error {
	result, err := s.db.Exec(`
INSERT INTO videos (userId, title, description, filePath, fileName, fileSize, duration, thumbnail) 
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		video.UserID, video.Title, video.Description, video.FilePath,
		video.FileName, video.FileSize, video.Duration, video.Thumbnail)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	video.ID = int(id)
	return nil
}

func (s *Store) UpdateVideo(video *types.Video) error {
	_, err := s.db.Exec(`
UPDATE videos 
SET title = ?, description = ?, duration = ?, thumbnail = ?, updatedAt = NOW() 
WHERE id = ?`,
		video.Title, video.Description, video.Duration, video.Thumbnail, video.ID)
	return err
}

func (s *Store) DeleteVideo(id int) error {
	_, err := s.db.Exec("DELETE FROM videos WHERE id = ?", id)
	return err
}

func scanRowIntoVideo(rows *sql.Rows) (*types.Video, error) {
	video := new(types.Video)

	var duration sql.NullFloat64
	var thumbnail sql.NullString
	err := rows.Scan(
		&video.ID,
		&video.UserID,
		&video.Title,
		&video.Description,
		&video.FilePath,
		&video.FileName,
		&video.FileSize,
		&duration,
		&thumbnail,
		&video.CreatedAt,
		&video.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if duration.Valid {
		video.Duration = duration.Float64
	}
	if thumbnail.Valid {
		video.Thumbnail = thumbnail.String
	}

	return video, nil
}
