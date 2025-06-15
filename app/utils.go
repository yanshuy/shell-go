package main

import (
	"fmt"
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


type RedirectionHandler struct {
	fds map[int]*os.File
}

func NewRedirectionHandler() *RedirectionHandler {
	return &RedirectionHandler{
		fds: map[int]*os.File{
			0: os.Stdin,
			1: os.Stdout,
			2: os.Stderr,
		},
	}
}

func (rh *RedirectionHandler) Close() error {
	var lastErr error
	for fd, file := range rh.fds {
		if fd > 2 && file != nil {
			if err := file.Close(); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

func (rh *RedirectionHandler) GetFD(fd int) *os.File {
	if file, ok := rh.fds[fd]; ok {
		return file
	}
	return nil
}

func (rh *RedirectionHandler) SetFD(fd int, file *os.File) {
	rh.fds[fd] = file
}

func getFileFromFD(fd int) *os.File {
	switch fd {
	case -1:
		devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		return devNull
	case 0:
		return os.Stdin
	case 1:
		return os.Stdout
	case 2:
		return os.Stderr
	default:
		fileName := fmt.Sprintf("/dev/fd/%d", fd)
		return os.NewFile(uintptr(fd), fileName)
	}
}

func HandleRedirections(cmdWR CommandWRedirections) (*Command, error) {
	var pcommand = cmdWR.Command
	for _, redir := range cmdWR.Redirections {
		switch redir.Operator {
		case ">", ">>":
			sourceFD, _ := redir.Source.(int)

			var destFile *os.File
			var err error
			if destFD, ok := redir.Destination.(int); ok {
				destFile = getFileFromFD(destFD)
			} else {
				filename := redir.Destination.(string)
				flags := os.O_WRONLY | os.O_CREATE
				if redir.Operator == ">>" {
					flags |= os.O_APPEND
				} else {
					flags |= os.O_TRUNC
				}
				destFile, err = os.OpenFile(filename, flags, 0644)
				if err != nil {
					return pcommand, fmt.Errorf("cannot open file %s: %v", filename, err)
				}
			}
			pcommand.redirHandler.SetFD(sourceFD, destFile)

		case "<":
			destFD, _ := redir.Destination.(int)

			var sourceFile *os.File
			var err error
			if sourceFD, ok := redir.Source.(int); ok {
				sourceFile = getFileFromFD(sourceFD)
			} else {
				filename := redir.Source.(string)
				sourceFile, err = os.OpenFile(filename, os.O_RDONLY, 0)
				if err != nil {
					return pcommand, fmt.Errorf("cannot open file %s: %v", filename, err)
				}
			}
			pcommand.redirHandler.SetFD(destFD, sourceFile)

		case "<<":
			return rp.handleHeredoc(redir)
		case "<<<":
			return rp.handleHereString(redir)
		case "<>":
			return pcommand, fmt.Errorf("unsupported redirection operator: %s", redir.Operator)
		default:
			return pcommand, fmt.Errorf("unsupported redirection operator: %s", redir.Operator)
		}
	}
	return pcommand, nil
}
