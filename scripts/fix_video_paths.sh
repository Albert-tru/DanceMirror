#!/bin/bash

# 修复数据库中的视频路径
# 将完整文件系统路径转换为Web可访问路径

echo "🔧 Fixing video paths in database..."

# 使用 Go 程序更新数据库
go run << 'GO_EOF'
package main

import (
        "fmt"
        "log"
        "strings"
        
        "github.com/Albert-tru/DanceMirror/config"
        "github.com/Albert-tru/DanceMirror/db"
        "github.com/Albert-tru/DanceMirror/types"
)

func main() {
        config.Envs = config.Config{
                DBDriver:      "sqlite3",
                DBSource:      "./dancemirror.db",
                UploadDir:     "./uploads",
                MaxUploadSize: 524288000,
        }
        
        database, err := db.NewSQLiteStorage()
        if err != nil {
                log.Fatal("Failed to connect to database:", err)
        }
        
        var videos []types.Video
        if err := database.DB().Find(&videos).Error; err != nil {
                log.Fatal("Failed to fetch videos:", err)
        }
        
        updatedCount := 0
        for i := range videos {
                video := &videos[i]
                oldPath := video.FilePath
                
                // 如果路径包含完整文件系统路径，转换为Web路径
                if strings.Contains(oldPath, "/uploads/") && !strings.HasPrefix(oldPath, "/uploads/") {
                        // 提取文件名
                        parts := strings.Split(oldPath, "/uploads/")
                        if len(parts) == 2 {
                                video.FilePath = "/uploads/" + parts[1]
                                
                                if err := database.DB().Save(video).Error; err != nil {
                                        log.Printf("Failed to update video %d: %v", video.ID, err)
                                        continue
                                }
                                
                                fmt.Printf("✅ Updated video %d: %s -> %s\n", video.ID, oldPath, video.FilePath)
                                updatedCount++
                        }
                }
        }
        
        fmt.Printf("\n🎉 Updated %d video paths\n", updatedCount)
}
GO_EOF

echo "✅ Video paths fixed!"
