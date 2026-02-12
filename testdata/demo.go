package testdata

import "os"

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
