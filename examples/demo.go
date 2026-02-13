package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
)

func test() {
	f1, err1 := os.Open("1.txt")
	if err1 != nil {
		return err1
	}
	defer f1.Close()
	f2, err2 := os.Open("2.txt")
	if err2 != nil {
		// 根据实际上下文，此处应进行适当的错误处理（如返回错误、记录日志等）
		// 示例：return err2
	}
	defer f2.Close()
	_ = f1
	_ = err1
	_ = f2
	_ = err2
}
func secDemo(db *sql.DB, userInput string) {
	token := os.Getenv("API_TOKEN")
	if token == "" {
		log.Fatal("API_TOKEN environment variable is not set")
	} // 触发 Secret 检查
	//query := fmt.Sprintf("SELECT * FROM users WHERE name='%s'", userInput)
	//db.Query("SELECT * FROM users WHERE id = ?", userID) // 触发 SQLi 检查
	fmt.Println(token)
}
