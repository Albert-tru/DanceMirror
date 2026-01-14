package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/Albert-tru/DanceMirror/types"
	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type ESClient struct {
	client *elasticsearch.Client
	index  string
}

func NewESClient(address string) (*ESClient, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{address},
	}
	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &ESClient{client: es, index: "videos"}, nil
}

// IndexVideo 将视频数据写入/更新到 ES
func (s *ESClient) IndexVideo(ctx context.Context, video *types.Video) error {
	body, err := json.Marshal(video)
	if err != nil {
		return err
	}

	req := esapi.IndexRequest{
		Index:      s.index,
		DocumentID: fmt.Sprintf("%d", video.ID),
		Body:       bytes.NewReader(body),
		Refresh:    "true", // 开发环境为了立即搜索到，生产环境可去掉
	}

	res, err := req.Do(ctx, s.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error indexing video ID=%d: %s", video.ID, res.String())
	}
	return nil
}

// SearchVideos 核心搜索逻辑
func (s *ESClient) SearchVideos(ctx context.Context, query string, page, pageSize int) ([]int, error) {
	var buf bytes.Buffer

	// 构建 ES 查询 DSL
	// 搜索 title, description, tags 字段
	queryMap := map[string]interface{}{
		"from": (page - 1) * pageSize,
		"size": pageSize,
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  query,
				"fields": []string{"title^3", "tags^2", "description"}, // 权重：标题>标签>描述
			},
		},
	}

	if err := json.NewEncoder(&buf).Encode(queryMap); err != nil {
		return nil, err
	}

	res, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(s.index),
		s.client.Search.WithBody(&buf),
		s.client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// 解析结果拿到 ID 列表
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}

	var ids []int
	if hits, ok := r["hits"].(map[string]interface{}); ok {
		if hitsList, ok := hits["hits"].([]interface{}); ok {
			for _, hit := range hitsList {
				hitMap := hit.(map[string]interface{})
				idStr := hitMap["_id"].(string)
				var id int
				fmt.Sscanf(idStr, "%d", &id)
				ids = append(ids, id)
			}
		}
	}
	return ids, nil
}
