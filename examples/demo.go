package main

import (
	"fmt"
	"os"
)

func test1() {
	open, err := os.Open("a.txt")
	_ = open
	_ = err
}

func someFunc() error {
	f, err := os.Open("config")
	_ = f
	_ = err
	return nil
	//return err // 开发者已经把错误传给上层了，这是合法的！
}
func nilDemo() {
	f, err := os.Open("none.txt")
	fmt.Println(f.Name()) // 在检查 err 之前就用了 f，触发 NilPointer 检查！
	_ = err
}

func ddd() {
	f, err := os.Open("config.txt")
	fmt.Println(f.Name())
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	str := []string{"11", "22", "33"}
	str = append([]string{str[0]}, str...)
	fmt.Println(str)
}
