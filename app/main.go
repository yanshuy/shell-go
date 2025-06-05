package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/term"
)


func main() {
	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		fmt.Println("Stdin is not a terminal")
		os.Exit(1)
	}

	prevState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Println("Error setting raw mode:", err)
		os.Exit(1)
	}
	defer term.Restore(int(os.Stdin.Fd()), prevState)

	t := term.NewTerminal(os.Stdin, "$ ")

	
	
	for {
		line, err := t.ReadLine()
		if err != nil {
			if err == io.EOF {
				t.Write([]byte("exit\n"))
				return
			}
			err := fmt.Sprintf("Error reading line: %v\n", err)
			t.Write([]byte(err))
			return
		}
		
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		command, redirects, err := ParseCommand(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %s\n", err.Error())
		}
		// fmt.Printf("parsed %#v\n%#v\n", command, redirects)
		
		outputStream := os.Stdout
		errorStream := os.Stderr
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
		case "history":
			output, cmdErr = historyCmd(t.History, command.Options, command.Args)
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

		if outputStream != os.Stdout {
			outputStream.Close()
		}
		if errorStream != os.Stderr {
			errorStream.Close()
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
	str := strings.Join(args, " ") + string(delimiter)
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
		err = fmt.Errorf("%s: not found", arg)
	}
	return output, err
}

func PwdCmd(options []string, args []string) (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("pwd: error: %v", err)
	}
	return pwd + "\n", nil
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

func historyCmd(h term.History, options []string, args []string) (string, error) {
	var builder strings.Builder
	var offset = 0

	if len(args) == 1 {
		num, err := strconv.Atoi(args[0])
		offset = h.Len() - num
		if err != nil {
			return "", fmt.Errorf("%s: invalid argument expected a number", args[0])
		}
	}
	for i := offset; i < h.Len(); i++ {
		builder.WriteString(fmt.Sprintf("%d  %s", i+1, h.At(i)))
	}
	return builder.String(), nil
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
