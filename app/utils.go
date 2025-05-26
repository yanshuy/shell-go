package main

import (
	"os"
	"strings"
)

var PATH = os.Getenv("PATH")
var paths = strings.Split(PATH, ":")

func findInPath(bin string) (string, bool) {
	for _, path := range paths {
		file := path + "/" + bin
		if _, err := os.Stat(file); err == nil {
			return file, true
		}
	}
	return "", false
}
