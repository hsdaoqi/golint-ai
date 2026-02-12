package testdata

import "os"

func ErrorTrigger() {
	f, err := os.Open("must_error.txt")
	_ = f
	// 故意不写 if err != nil，也不写 return err
}
