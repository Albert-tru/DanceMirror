package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	passwords := []string{
		"Dance@2025",
		"Dance%402025",
		"",
	}

	users := []string{"dmuser", "ecomuser", "root"}

	for _, user := range users {
		for _, pass := range passwords {
			dsn := fmt.Sprintf("%s:%s@tcp(localhost:3306)/dancemirror?charset=utf8mb4&parseTime=True", user, pass)
			db, err := sql.Open("mysql", dsn)
			if err != nil {
				continue
			}
			err = db.Ping()
			if err == nil {
				fmt.Printf("✅ 成功连接: 用户=%s, 密码=%s\n", user, pass)
				db.Close()
				return
			}
			db.Close()
		}
	}
	log.Println("❌ 所有连接尝试都失败了")
}
