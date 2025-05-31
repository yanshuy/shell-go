package main

import (
	"os"
	"path/filepath"
	"strings"
)

var PATH = os.Getenv("PATH")
var paths = strings.Split(PATH, ":")

func findInPath(cmd string) (string, bool) {
	for _, path := range paths {
		filePath := filepath.Join(path, cmd)
		fileInfo, err := os.Stat(filePath)
		if err == nil && fileInfo.Mode().Perm()&0111 != 0 {
			return filePath, true
		}
	}
	return "", false
}

func handleRedirection(redirects []Redirection) (outputStream *os.File, errorStream *os.File, err error) {
	for _, redirect := range redirects {
		if redirect.File[0] == '~' {
			HOME := os.Getenv("HOME")
			redirect.File = strings.Replace(redirect.File, "~", HOME, 1)
		}
		switch redirect.Operator {
		case ">", "1>":
			file, err := os.Create(redirect.File)
			if err != nil {
				return nil, nil, err
			}
			outputStream = file

		case "2>":
			file, err := os.Create(redirect.File)
			if err != nil {
				return nil, nil, err
			}
			errorStream = file

		case ">>", "1>>":
			file, err := os.Open(redirect.File)
			if err != nil {
				return nil, nil, err
			}
			outputStream = file

		case "2>>":
			file, err := os.Open(redirect.File)
			if err != nil {
				return nil, nil, err
			}
			errorStream = file
		}

	}
	return outputStream, errorStream, nil
}
