package video

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Albert-tru/DanceMirror/service/mq"
	"github.com/Albert-tru/DanceMirror/types"
	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
	mq *mq.RabbitMQClient
}

func NewStore(db *gorm.DB, mq *mq.RabbitMQClient) *Store {
	return &Store{db: db, mq: mq}
}

func (s *Store) GetVideosByIDs(ctx context.Context, ids []int) ([]*types.Video, error) {
	if len(ids) == 0 {
		return []*types.Video{}, nil
	}

	var videos []types.Video
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&videos).Error; err != nil {
		return nil, err
	}

	videoMap := make(map[int]*types.Video)
	for i := range videos {
		videoMap[videos[i].ID] = &videos[i]
	}

	orderedVideos := make([]*types.Video, 0, len(ids))
	for _, id := range ids {
		if v, exists := videoMap[id]; exists {
			orderedVideos = append(orderedVideos, v)
		}
	}
	return orderedVideos, nil
}

func (s *Store) GetVideoByID(ctx context.Context, id int) (*types.Video, error) {
	var video *types.Video
	result := s.db.WithContext(ctx).First(&video, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return video, nil
}

func (s *Store) GetVideos(ctx context.Context, userID int64) ([]*types.Video, error) {
	var videos []types.Video
	result := s.db.WithContext(ctx).Where("userId = ?", userID).Order("createdAt DESC").Find(&videos)

	videoPtrs := make([]*types.Video, len(videos))
	for i := range videos {
		videoPtrs[i] = &videos[i]
	}
	return videoPtrs, result.Error
}

func (s *Store) CreateVideo(ctx context.Context, video *types.Video) error {
	err := s.db.WithContext(ctx).Create(video).Error
	if err != nil {
		return err
	}

	if s.mq != nil {
		go func() {
			msg := mq.SyncVideoESMsg{VideoID: video.ID, Action: "index"}
			body, _ := json.Marshal(msg)
			_ = s.mq.Publish("video_sync_es_queue", body)
		}()
	}

	return nil
}

func (s *Store) UpdateVideo(ctx context.Context, video *types.Video) error {
	return s.db.WithContext(ctx).Save(video).Error
}

func (s *Store) DeleteVideo(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Delete(&types.Video{}, id).Error
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

func (s *Store) SearchVideos(ctx context.Context, keyword string, page, size int, sort string) ([]*types.Video, int64, error) {
	var videos []types.Video
	var total int64
	offset := (page - 1) * size

	query := s.db.WithContext(ctx).Model(&types.Video{})
	if keyword != "" {
		query = query.Where("title LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderClause := "createdAt DESC"
	switch sort {
	case "date_asc":
		orderClause = "createdAt ASC"
	case "title_asc":
		orderClause = "title ASC"
	case "title_desc":
		orderClause = "title DESC"
	}

	err := query.Order(orderClause).Limit(size).Offset(offset).Find(&videos).Error
	if err != nil {
		return nil, 0, err
	}

	videoPtrs := make([]*types.Video, len(videos))
	for i := range videos {
		videoPtrs[i] = &videos[i]
	}
	return videoPtrs, total, nil
}

func (s *Store) CreateUploadEvent(ctx context.Context, userID int64, videoID *int, status string, fileSize int64, errorCode, requestID string) error {
	return s.db.WithContext(ctx).Exec(`
INSERT INTO upload_events (userId, videoId, status, fileSize, errorCode, requestId)
VALUES (?, ?, ?, ?, ?, ?)
`, userID, videoID, status, fileSize, errorCode, requestID).Error
}

func (s *Store) CreateAITask(ctx context.Context, userID int64, videoID int, status, requestID string) error {
	return s.db.WithContext(ctx).Exec(`
INSERT INTO ai_tasks (userId, videoId, status, requestId)
VALUES (?, ?, ?, ?)
`, userID, videoID, status, requestID).Error
}

func (s *Store) MarkAITaskFailed(ctx context.Context, userID int64, videoID int, reason string) error {
	return s.db.WithContext(ctx).Exec(`
UPDATE ai_tasks
SET status = 'failed', errorReason = ?, durationMs = TIMESTAMPDIFF(MICROSECOND, createdAt, NOW())/1000
WHERE userId = ? AND videoId = ?
ORDER BY id DESC
LIMIT 1
`, reason, userID, videoID).Error
}

func (s *Store) MarkAITaskSuccess(ctx context.Context, userID int64, videoID int) error {
	return s.db.WithContext(ctx).Exec(`
UPDATE ai_tasks
SET status = 'success', errorReason = NULL, durationMs = TIMESTAMPDIFF(MICROSECOND, createdAt, NOW())/1000
WHERE userId = ? AND videoId = ?
ORDER BY id DESC
LIMIT 1
`, userID, videoID).Error
}
