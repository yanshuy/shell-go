package main

import (
	"os"
	"strings"
)

var PATH = os.Getenv("PATH")
var paths = strings.Split(PATH, ":")

func findInPath(cmd string) (string, bool) {
	for _, path := range paths {
		file := path + "/" + cmd
		fileInfo, err := os.Stat(file)
		if err == nil && fileInfo.Mode().Perm()&0111 != 0 {
			return file, true
		}
	}
	return "", false
}
