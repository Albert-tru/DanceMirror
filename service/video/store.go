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
	return &Store{
		db: db,
		mq: mq,
	}
}

func (s *Store) GetVideosByIDs(ctx context.Context, ids []int) ([]*types.Video, error) {
	if len(ids) == 0 {
		return []*types.Video{}, nil
	}

	var videos []types.Video
	// 使用 IN 查询
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&videos).Error; err != nil {
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
	// 先存入数据库
	err := s.db.WithContext(ctx).Create(video).Error
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

func (s *Store) UpdateVideo(ctx context.Context, video *types.Video) error {
	return s.db.WithContext(ctx).Save(video).Error
}

func (s *Store) DeleteVideo(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Delete(&types.Video{}, id).Error // 软删除
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
	// 1. 定义结果容器和初始变量
	var videos []types.Video
	var total int64
	offset := (page - 1) * size // 计算分页偏移量 (核心分页逻辑)

	// 2. 构建基础查询 (GORM 链式调用)
	query := s.db.WithContext(ctx).Model(&types.Video{})

	// 3. 动态添加搜索条件 (SQL: WHERE title LIKE %kw% OR description LIKE %kw%)
	if keyword != "" {
		query = query.Where("title LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 4. 获取总数 (用于前端显示 "共 100 条")
	// 注意：Count 必须在 Limit/Offset 之前执行
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 5. 处理排序逻辑 (白名单机制，防止 SQL 注入)
	orderClause := "createdAt DESC" // 默认按时间倒序
	switch sort {
	case "date_asc":
		orderClause = "createdAt ASC"
	case "title_asc":
		orderClause = "title ASC"
	case "title_desc":
		orderClause = "title DESC"
	}

	// 6. 执行最终查询 (Search + Sort + Page)
	// SQL: SELECT * FROM videos WHERE ... ORDER BY ... LIMIT 20 OFFSET 0
	err := query.Order(orderClause).Limit(size).Offset(offset).Find(&videos).Error
	if err != nil {
		return nil, 0, err
	}

	// 7. 转换返回值类型 (由 []Video 转 [] *Video)
	videoPtrs := make([]*types.Video, len(videos))
	for i := range videos {
		videoPtrs[i] = &videos[i]
	}

	return videoPtrs, total, nil
}
