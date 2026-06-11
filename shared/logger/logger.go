package logger

import "fmt"

func Info(format string, a ...interface{}) {
	fmt.Printf("[INFO] " + format + "\n", a...)
}

func Error(format string, a ...interface{}) {
	fmt.Printf("[ERROR] " + format + "\n", a...)
}
