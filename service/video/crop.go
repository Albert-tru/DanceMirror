package video

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings" // ✅ 新增引用
	"time"

	"github.com/Albert-tru/DanceMirror/config"
	"github.com/Albert-tru/DanceMirror/utils/logger"
"github.com/Albert-tru/DanceMirror/types"
)

// CropParams 视频裁剪参数

// CropVideo 使用 FFmpeg 裁剪视频
func CropVideo(inputPath string, outputPath string, params types.CropParams) error {
	// 验证裁剪参数
	if params.Width <= 0 || params.Height <= 0 {
		return fmt.Errorf("invalid crop dimensions: width=%d, height=%d", params.Width, params.Height)
	}

	if params.X < 0 || params.Y < 0 {
		return fmt.Errorf("invalid crop position: x=%d, y=%d", params.X, params.Y)
	}

	// ✅ 修复：如果是 URL (MinIO)，跳过本地文件检查
	// FFmpeg 支持 HTTP/HTTPS 协议，不需要 os.Stat 检查
	if !strings.HasPrefix(inputPath, "http://") && !strings.HasPrefix(inputPath, "https://") {
		// 确保输入文件存在 (仅本地文件模式下检查)
		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			return fmt.Errorf("input file does not exist: %s", inputPath)
		}
	}

	// 确保输出目录存在
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// 构建 FFmpeg 裁剪命令
	// crop=width:height:x:y
	cropFilter := fmt.Sprintf("crop=%d:%d:%d:%d", params.Width, params.Height, params.X, params.Y)

	// FFmpeg 命令参数
	args := []string{
		"-i", inputPath, // 输入文件 (可以是本地路径，也可以是 URL)
		"-vf", cropFilter, // 视频裁剪过滤器
		"-c:a", "copy", // 音频直接复制，不重新编码
		"-c:v", "libx264", // 视频使用 H.264 编码
		"-preset", "ultrafast", // 编码速度（fast, medium, slow）
		"-crf", "23", // 恒定质量模式，23 是默认值（18-28，越小质量越高）
		"-movflags", "+faststart", // 优化 MP4 流式播放
		"-y",       // 覆盖输出文件
		outputPath, // 输出文件
	}

	logger.Info(fmt.Sprintf("Starting FFmpeg crop: %s -> %s (crop=%dx%d at %d,%d)",
		inputPath, outputPath, params.Width, params.Height, params.X, params.Y))

	// 执行 FFmpeg 命令
	cmd := exec.Command("ffmpeg", args...)

	// 捕获错误输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error(fmt.Sprintf("FFmpeg crop failed: %v\nOutput: %s", err, string(output)))
		return fmt.Errorf("ffmpeg crop failed: %v", err)
	}

	logger.Info(fmt.Sprintf("FFmpeg crop completed successfully: %s", outputPath))
	return nil
}

// CheckFFmpegAvailable 检查 FFmpeg 是否可用
func CheckFFmpegAvailable() bool {
	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		logger.Warn("FFmpeg is not available on this system")
		return false
	}
	return true
}

// GenerateCroppedFilename 生成裁剪后的文件名
func GenerateCroppedFilename(originalFilename string, userID int) string {
	ext := filepath.Ext(originalFilename)
	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("%d_cropped_%s%s", userID, timestamp, ext)
}

// CleanupTempFile 清理临时文件
func CleanupTempFile(path string) {
	if err := os.Remove(path); err != nil {
		logger.Warn(fmt.Sprintf("Failed to cleanup temp file %s: %v", path, err))
	} else {
		logger.Info(fmt.Sprintf("Cleaned up temp file: %s", path))
	}
}

// ParseCropParams 从字符串解析裁剪参数
func ParseCropParams(xStr, yStr, wStr, hStr string) (types.CropParams, error) {
	x, err := strconv.Atoi(xStr)
	if err != nil {
		return types.CropParams{}, fmt.Errorf("invalid cropX parameter: %v", err)
	}

	y, err := strconv.Atoi(yStr)
	if err != nil {
		return types.CropParams{}, fmt.Errorf("invalid cropY parameter: %v", err)
	}

	w, err := strconv.Atoi(wStr)
	if err != nil {
		return types.CropParams{}, fmt.Errorf("invalid cropW parameter: %v", err)
	}

	h, err := strconv.Atoi(hStr)
	if err != nil {
		return types.CropParams{}, fmt.Errorf("invalid cropH parameter: %v", err)
	}

	return types.CropParams{
		X:      x,
		Y:      y,
		Width:  w,
		Height: h,
	}, nil
}

// GetTempCropDir 获取裁剪临时目录
func GetTempCropDir() string {
	return filepath.Join(config.Envs.UploadDir, "crop_temp")
}

// GetCroppedOutputDir 获取裁剪输出目录
func GetCroppedOutputDir() string {
	return filepath.Join(config.Envs.UploadDir, "cropped")
}
