package video

import (
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
	return &Store{
		db: db,
		mq: mq,
	}
}

func (s *Store) GetVideosByIDs(ids []int) ([]*types.Video, error) {
	if len(ids) == 0 {
		return []*types.Video{}, nil
	}

	var videos []types.Video
	// 使用 IN 查询
	if err := s.db.Where("id IN ?", ids).Find(&videos).Error; err != nil {
		return nil, err
	}

	// 建立 ID -> Video 的映射，方便按 ids 的顺序重排
	videoMap := make(map[int]*types.Video)
	for i := range videos {
		videoMap[videos[i].ID] = &videos[i]
	}

	// 按照传入 ids 的顺序构造结果（保持 ES 的搜索相关度顺序）
	orderedVideos := make([]*types.Video, 0, len(ids))
	for _, id := range ids {
		if v, exists := videoMap[id]; exists {
			orderedVideos = append(orderedVideos, v)
		}
	}

	return orderedVideos, nil
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
	result := s.db.Where("userId = ?", userID).Order("createdAt DESC").Find(&videos)

	videoPtrs := make([]*types.Video, len(videos))
	for i := range videos {
		videoPtrs[i] = &videos[i]
	}

	return videoPtrs, result.Error
}

func (s *Store) CreateVideo(video *types.Video) error {
	// 先存入数据库
	err := s.db.Create(video).Error
	if err != nil {
		return err
	}

	// 如果 MQ 客户端存在，则异步发送 ES 同步消息
	if s.mq != nil {
		go func() {
			msg := mq.SyncVideoESMsg{VideoID: video.ID, Action: "index"}
			body, _ := json.Marshal(msg)
			s.mq.Publish("video_sync_es_queue", body)
		}()
	}

	return nil
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
