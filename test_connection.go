package main

import (
    "fmt"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
)

func main() {
    dsn := "root:MySQL666@tcp(127.0.0.1:3306)/dancemirror_test?charset=utf8mb4&parseTime=True&loc=Local"
    fmt.Printf("尝试连接: %s\n", dsn)
    
    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    if err != nil {
        fmt.Printf("❌ 连接失败: %v\n", err)
        return
    }
    fmt.Println("✅ GORM 连接成功！")
    
    var version string
    db.Raw("SELECT VERSION()").Scan(&version)
    fmt.Printf("数据库版本: %s\n", version)
    
    // 测试 DELETE 操作
    result := db.Exec("DELETE FROM users")
    if result.Error != nil {
        fmt.Printf("❌ DELETE 失败: %v\n", result.Error)
    } else {
        fmt.Printf("✅ DELETE 成功！影响行数: %d\n", result.RowsAffected)
    }
}
