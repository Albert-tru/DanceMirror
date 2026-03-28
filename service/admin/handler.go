package admin

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Albert-tru/DanceMirror/service/auth"
	"github.com/Albert-tru/DanceMirror/types"
	"github.com/Albert-tru/DanceMirror/utils"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type Handler struct {
	db        *gorm.DB
	userStore types.UserStore
}

func NewHandler(db *gorm.DB, userStore types.UserStore) *Handler {
	return &Handler{db: db, userStore: userStore}
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	admin := router.PathPrefix("/admin").Subrouter()
	admin.HandleFunc("/users/overview", auth.WithAdminAuth(h.handleUsersOverview, h.userStore)).Methods(http.MethodGet)
	admin.HandleFunc("/stats/videos", auth.WithAdminAuth(h.handleVideoStats, h.userStore)).Methods(http.MethodGet)
	admin.HandleFunc("/stats/ai", auth.WithAdminAuth(h.handleAIStats, h.userStore)).Methods(http.MethodGet)
	admin.HandleFunc("/troubleshoot/errors", auth.WithAdminAuth(h.handleTroubleshootErrors, h.userStore)).Methods(http.MethodGet)
	admin.HandleFunc("/troubleshoot/failed-tasks", auth.WithAdminAuth(h.handleFailedTasks, h.userStore)).Methods(http.MethodGet)
}

type userOverviewItem struct {
	UserID           int64   `json:"userId"`
	Phone            string  `json:"phone"`
	Nickname         string  `json:"nickname"`
	CreatedAt        string  `json:"createdAt"`
	LastLoginAt      *string `json:"lastLoginAt,omitempty"`
	AccountStatus    string  `json:"accountStatus"`
	UploadVideoCount int64   `json:"uploadVideoCount"`
}

func (h *Handler) handleUsersOverview(w http.ResponseWriter, r *http.Request) {
	var rows []userOverviewItem
	query := `
SELECT
  u.id AS user_id,
  u.phone,
  COALESCE(NULLIF(u.nickname, ''), TRIM(CONCAT(COALESCE(u.firstName,''), ' ', COALESCE(u.lastName,'')))) AS nickname,
  DATE_FORMAT(u.createdAt, '%Y-%m-%d %H:%i:%s') AS created_at,
  CASE WHEN u.lastLoginAt IS NULL THEN NULL ELSE DATE_FORMAT(u.lastLoginAt, '%Y-%m-%d %H:%i:%s') END AS last_login_at,
  COALESCE(NULLIF(u.accountStatus,''), 'active') AS account_status,
  COUNT(v.id) AS upload_video_count
FROM users u
LEFT JOIN videos v ON v.userId = u.id AND v.deletedAt IS NULL
WHERE u.deletedAt IS NULL
GROUP BY u.id, u.phone, u.nickname, u.firstName, u.lastName, u.createdAt, u.lastLoginAt, u.accountStatus
ORDER BY u.createdAt DESC`
	if err := h.db.Raw(query).Scan(&rows).Error; err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	_ = utils.WriteJSON(w, http.StatusOK, rows)
}

type videoStatItem struct {
	UserID            int64   `json:"userId"`
	Phone             string  `json:"phone"`
	Nickname          string  `json:"nickname"`
	TotalUploadCount  int64   `json:"totalUploadCount"`
	LatestUploadAt    *string `json:"latestUploadAt,omitempty"`
	TotalFileSizeMB   float64 `json:"totalFileSizeMB"`
	FailedUploadCount int64   `json:"failedUploadCount"`
	CropRequestCount  int64   `json:"cropRequestCount"`
}

func (h *Handler) handleVideoStats(w http.ResponseWriter, r *http.Request) {
	var rows []videoStatItem
	query := `
SELECT
  u.id AS user_id,
  u.phone,
  COALESCE(NULLIF(u.nickname, ''), TRIM(CONCAT(COALESCE(u.firstName,''), ' ', COALESCE(u.lastName,'')))) AS nickname,
  (
    SELECT COUNT(1)
    FROM videos v
    WHERE v.userId = u.id AND v.deletedAt IS NULL
  ) AS total_upload_count,
  (
    SELECT CASE WHEN MAX(v.createdAt) IS NULL THEN NULL ELSE DATE_FORMAT(MAX(v.createdAt), '%Y-%m-%d %H:%i:%s') END
    FROM videos v
    WHERE v.userId = u.id AND v.deletedAt IS NULL
  ) AS latest_upload_at,
  ROUND(
    COALESCE((
      SELECT SUM(v.fileSize)
      FROM videos v
      WHERE v.userId = u.id AND v.deletedAt IS NULL
    ), 0) / 1048576.0,
    2
  ) AS total_file_size_mb,
  (
    SELECT COUNT(1)
    FROM upload_events ue
    WHERE ue.userId = u.id AND ue.status = 'failed'
  ) AS failed_upload_count,
  (
    SELECT COUNT(1)
    FROM upload_events ue
    WHERE ue.userId = u.id AND ue.status = 'crop_requested'
  ) AS crop_request_count
FROM users u
WHERE u.deletedAt IS NULL
ORDER BY total_upload_count DESC, total_file_size_mb DESC`
	if err := h.db.Raw(query).Scan(&rows).Error; err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	_ = utils.WriteJSON(w, http.StatusOK, rows)
}

type aiStatItem struct {
	UserID        int64   `json:"userId"`
	Phone         string  `json:"phone"`
	Nickname      string  `json:"nickname"`
	LaunchCount   int64   `json:"launchCount"`
	FailCount     int64   `json:"failCount"`
	FailRate      float64 `json:"failRate"`
	LastStatus    *string `json:"lastStatus,omitempty"`
	LastError     *string `json:"lastError,omitempty"`
	LastUpdatedAt *string `json:"lastUpdatedAt,omitempty"`
}

func (h *Handler) handleAIStats(w http.ResponseWriter, r *http.Request) {
	var rows []aiStatItem
	query := `
SELECT
  u.id AS user_id,
  u.phone,
  COALESCE(NULLIF(u.nickname, ''), TRIM(CONCAT(COALESCE(u.firstName,''), ' ', COALESCE(u.lastName,'')))) AS nickname,
  COALESCE(COUNT(t.id), 0) AS launch_count,
  COALESCE(SUM(CASE WHEN t.status = 'failed' THEN 1 ELSE 0 END), 0) AS fail_count,
  CASE WHEN COUNT(t.id) = 0 THEN 0 ELSE ROUND(SUM(CASE WHEN t.status = 'failed' THEN 1 ELSE 0 END) * 100.0 / COUNT(t.id), 2) END AS fail_rate,
  (SELECT t2.status FROM ai_tasks t2 WHERE t2.userId = u.id ORDER BY t2.updatedAt DESC LIMIT 1) AS last_status,
  (SELECT t2.errorReason FROM ai_tasks t2 WHERE t2.userId = u.id ORDER BY t2.updatedAt DESC LIMIT 1) AS last_error,
  (SELECT DATE_FORMAT(t2.updatedAt, '%Y-%m-%d %H:%i:%s') FROM ai_tasks t2 WHERE t2.userId = u.id ORDER BY t2.updatedAt DESC LIMIT 1) AS last_updated_at
FROM users u
LEFT JOIN ai_tasks t ON t.userId = u.id
WHERE u.deletedAt IS NULL
GROUP BY u.id, u.phone, u.nickname, u.firstName, u.lastName
ORDER BY launch_count DESC, fail_count DESC`
	if err := h.db.Raw(query).Scan(&rows).Error; err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	_ = utils.WriteJSON(w, http.StatusOK, rows)
}

type apiErrorItem struct {
	ID         int64  `json:"id"`
	RequestID  string `json:"requestId"`
	UserID     *int64 `json:"userId,omitempty"`
	VideoID    *int   `json:"videoId,omitempty"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"statusCode"`
	ErrorCode  string `json:"errorCode"`
	Message    string `json:"message"`
	CreatedAt  string `json:"createdAt"`
}

func (h *Handler) handleTroubleshootErrors(w http.ResponseWriter, r *http.Request) {
	hours := 24
	if v := r.URL.Query().Get("hours"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 168 {
			hours = parsed
		}
	}

	query := `
SELECT
  id,
  requestId AS request_id,
  userId AS user_id,
  videoId AS video_id,
  method,
  path,
  statusCode AS status_code,
  errorCode AS error_code,
  COALESCE(message, '') AS message,
  DATE_FORMAT(createdAt, '%Y-%m-%d %H:%i:%s') AS created_at
FROM api_error_logs
WHERE createdAt >= DATE_SUB(NOW(), INTERVAL ? HOUR)`
	args := []interface{}{hours}

	if requestID := r.URL.Query().Get("requestId"); requestID != "" {
		query += " AND requestId = ?"
		args = append(args, requestID)
	}
	if userID := r.URL.Query().Get("userId"); userID != "" {
		query += " AND userId = ?"
		args = append(args, userID)
	}
	if videoID := r.URL.Query().Get("videoId"); videoID != "" {
		query += " AND videoId = ?"
		args = append(args, videoID)
	}
	query += " ORDER BY createdAt DESC LIMIT 300"

	var rows []apiErrorItem
	if err := h.db.Raw(query, args...).Scan(&rows).Error; err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	_ = utils.WriteJSON(w, http.StatusOK, rows)
}

type failedTaskItem struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"userId"`
	VideoID     int    `json:"videoId"`
	Status      string `json:"status"`
	DurationMs  int64  `json:"durationMs"`
	ErrorReason string `json:"errorReason"`
	RequestID   string `json:"requestId"`
	UpdatedAt   string `json:"updatedAt"`
}

func (h *Handler) handleFailedTasks(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	var rows []failedTaskItem
	query := `
SELECT
  id,
  userId AS user_id,
  videoId AS video_id,
  status,
  durationMs AS duration_ms,
  COALESCE(errorReason, '') AS error_reason,
  COALESCE(requestId, '') AS request_id,
  DATE_FORMAT(updatedAt, '%Y-%m-%d %H:%i:%s') AS updated_at
FROM ai_tasks
WHERE status = 'failed'
ORDER BY updatedAt DESC
LIMIT ?`
	if err := h.db.Raw(query, limit).Scan(&rows).Error; err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	_ = utils.WriteJSON(w, http.StatusOK, rows)
}

func MustAdminSeedSQL() string {
	return fmt.Sprintf("UPDATE users SET role='admin' WHERE id=1;")
}
