package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func RootDir() string {
	_, filename, _, _ := runtime.Caller(1)
	dir := filepath.Dir(filename)
	if fileExists(dir + "/.git") {
		return filepath.Clean(dir)
	}
	dir += "/"
	for i := 0; i < 3; i++ {
		dir = dir + "../"
		if fileExists(dir + ".git") {
			return filepath.Clean(dir)
		}
	}
	panic("Cannot find .git/")
}

func fileExists(file string) bool {
	if _, err := os.Stat(file); err == nil {
		return true
	}
	return false
}

func Dump(v interface{}) {
	bytes, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(bytes))
}
