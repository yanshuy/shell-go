package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	for {
		fmt.Fprint(os.Stdout, "$ ")

		rawInput, err := bufio.NewReader(os.Stdin).ReadString(byte(delimiter))
		if err != nil {
			if err == io.EOF {
				fmt.Println("exit")
				os.Exit(0)
			}
			fmt.Fprintln(os.Stderr, "error reading input:", err)
			os.Exit(1)
		}

		command, redirects, err := ParseCommand(rawInput)
		if err != nil {
			if err != ParseErrNoCommand {
				fmt.Fprintf(os.Stderr, "parse error: %s\n", err.Error())
			}
			continue
		}
		// fmt.Printf("parsed %#v\n%#v\n", command, redirects)

		outputStream := os.Stdout
		errorStream := os.Stderr
		defer outputStream.Close()
		defer errorStream.Close()

		if len(redirects) > 0 {
			var err error
			outputStream, errorStream, err = handleRedirection(redirects)
			if err != nil {
				fmt.Fprintf(errorStream, "%s\n", err.Error())
				continue
			}
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
		default:
			ExternalCmd(cmd, command.Args, outputStream, errorStream)
			continue
		}

		if cmdErr != nil {
			fmt.Fprintf(errorStream, "%s\n", cmdErr.Error())
		}
		if output != "" {
			if !strings.HasSuffix(output, "\n") {
				fmt.Fprintln(outputStream, output)
			} else {
				fmt.Fprint(outputStream, output)
			}
		}
	}
}

func ExitCmd(options []string, args []string) (err error) {
	code := 0
	if len(args) > 0 {
		code, err = strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid status expected a number: %s", err.Error())
		}
	}
	os.Exit(code)
	return nil
}

func EchoCmd(options []string, args []string) (string, error) {
	str := strings.Join(args, " ")
	return str, nil
}

func TypeCmd(options []string, args []string) (output string, err error) {
	if len(args) == 0 {
		return "", nil
	}

	for _, arg := range args {
		if _, ok := builtins[arg]; ok {
			output = fmt.Sprintf("%s is a shell builtin\n", arg)
			continue
		}

		if file, ok := findInPath(arg); ok {
			output = fmt.Sprintf("%s is %s\n", arg, file)
			continue
		}
		err = fmt.Errorf("%s: not found\n", arg)
	}
	return output, err
}

func PwdCmd(options []string, args []string) (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("pwd: error: %v", err)
	}
	return pwd, nil
}

func CdCmd(options []string, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if len(args) > 1 {
		return ErrTooManyArguments
	}

	dir := args[0]
	if len(dir) > 0 && dir[0] == '~' {
		HOME := os.Getenv("HOME")
		dir = strings.Replace(dir, "~", HOME, 1)
	}

	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("cd: %s: No such file or directory", dir)
	}
	return nil
}

func ExternalCmd(cmdName string, args []string, outputStream *os.File, errorStream *os.File) {
	if _, ok := findInPath(cmdName); !ok {
		fmt.Fprintf(errorStream, "%s: command not found", cmdName)
	}
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = outputStream
	cmd.Stderr = errorStream
	cmd.Run()
}
