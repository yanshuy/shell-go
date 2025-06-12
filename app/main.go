package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func main() {
	doneChan := make(chan bool)
	s := NewShell(doneChan)

	var input string
	for {
		line, err := s.ReadLine()
		if err != nil {
			if err == io.EOF {
				fmt.Fprint(s, "(Ctrl+D) received. Exiting\n")
			} else {
				fmt.Fprintf(s, "Error reading line: %w\n", err)
			}
			break
		}

		input = strings.TrimSpace(line)
		if line == "" || line[:len(line)-1] == "|" {
			continue
		}

		commands, err := Parse(input)
		if err == ErrUnexpectedEnd {
			continue
		}
		if err != nil {
			fmt.Fprintf(s, "parse error: %w\n", err)
		}
		// fmt.Printf("parsed %#v\n%#v\n", command, redirects)

		if len(commands) > 1 {
			s.ExecutePipeline(commands)
		}

		outputStream := os.Stdout
		errorStream := os.Stderr
		if len(redirects) > 0 {
			var err error
			outputStream, errorStream, err = handleRedirection(redirects)
			if err != nil {
				fmt.Fprintf(errorStream, "%s\n", err.Error())
				continue
			}
			defer outputStream.Close()
			defer errorStream.Close()
		}

		cmd := command.Name
		var output string = ""
		var cmdErr error

		switch cmd {
		case "exit":
			ExitCmd(command.Options, command.Args)
		case "echo":
			output, cmdErr = EchoCmd(command.Options, command.Args)
		case "type":
			output, cmdErr = TypeCmd(command.Options, command.Args)
		case "pwd":
			output, cmdErr = PwdCmd(command.Options, command.Args)
		case "cd":
			cmdErr = CdCmd(command.Options, command.Args)
		case "history":
			output, cmdErr = HistoryCmd(t.History, command.Options, command.Args)
		default:
			ExternalCmd(cmd, command.Args, outputStream, errorStream)
			continue
		}

		if cmdErr != nil {
			fmt.Fprintf(errorStream, "%s\n", cmdErr.Error())
		}
		//output should have the delimiter
		if output != "" {
			fmt.Fprint(outputStream, output)
		}
	}
	doneChan <- true
}

func executeCommand(command CommandWithIOSetup) {
	cmdName := command.Name

	var output string = ""
	var cmdErr error
	switch cmdName {
	case "exit":
		ExitCmd(command.Options, command.Args)
	case "echo":
		output, cmdErr = EchoCmd(command.Options, command.Args)
	case "type":
		output, cmdErr = TypeCmd(command.Options, command.Args)
	case "pwd":
		output, cmdErr = PwdCmd(command.Options, command.Args)
	case "cd":
		cmdErr = CdCmd(command.Options, command.Args)
	case "history":
		output, cmdErr = s.HistoryCmd(command.Options, command.Args)
	default:
		ExternalCmd(cmdName, command.Args, command.Stdout, command.Stderr)
	}

	if cmdErr != nil {
		fmt.Fprintf(command.Stderr, "%s\n", cmdErr.Error())
	}
	//output should have the delimiter
	if output != "" {
		fmt.Fprint(command.Stdout, output)
	}
}

func ExternalCmd(cmdName string, args []string, outputStream *os.File, errorStream *os.File) {
	if _, ok := findInPath(cmdName); !ok {
		fmt.Fprintf(errorStream, "%s: command not found\n", cmdName)
	}
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = outputStream
	cmd.Stderr = errorStream
	cmd.Run()
}
