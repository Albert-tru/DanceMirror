package video

import (
	"database/sql"

	"gorm.io/gorm"

	"github.com/Albert-tru/DanceMirror/types"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetVideoByID(id int) (*types.Video, error) {
	var video *types.Video
	result := s.db.First(&video, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return video, nil
}

func (s *Store) GetVideos(userID int) ([]*types.Video, error) {
	var videos []types.Video
	result := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&videos)

	videoPtrs := make([]*types.Video, len(videos))
	for i := range videos {
		videoPtrs[i] = &videos[i]
	}

	return videoPtrs, result.Error
}

func (s *Store) CreateVideo(video *types.Video) error {
	return s.db.Create(video).Error
}

func (s *Store) UpdateVideo(video *types.Video) error {
	return s.db.Save(video).Error
}

func (s *Store) DeleteVideo(id int) error {
	return s.db.Delete(&types.Video{}, id).Error // 软删除
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
