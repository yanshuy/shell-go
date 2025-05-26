package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var delimiter byte = '\n'

var builtinCmds = map[string]struct{}{
	"exit": {},
	"echo": {},
	"type": {},
	"pwd":  {},
	"cd":   {},
}

func ParseInput(input string) ([]string, error) {
	var argv []string

	currentArg := []byte{}
	for i := 0; input[i] != delimiter; i++ {
		if input[i] == '\'' {
			i++
			for input[i] != '\'' {
				if input[i] == delimiter {
					return nil, fmt.Errorf("no trailing single quote")
				}
				currentArg = append(currentArg, input[i])
				i++
			}
			if i+1 < len(input) && input[i+1] != '\'' {
				argv = append(argv, string(currentArg))
				currentArg = []byte{}
			}
			continue
		}
		if input[i] == '"' {
			i++
			for input[i] != '"' {
				if input[i] == delimiter {
					return nil, fmt.Errorf("no trailing double quote")
				}
				currentArg = append(currentArg, input[i])
				i++
			}
			if i+1 < len(input) && input[i+1] != '"' {
				argv = append(argv, string(currentArg))
				currentArg = []byte{}
			}
			continue
		}
		if input[i] == ' ' {
			if len(currentArg) > 0 {
				argv = append(argv, string(currentArg))
				currentArg = []byte{}
			}
			continue
		}
		currentArg = append(currentArg, input[i])
	}

	if len(currentArg) > 0 {
		argv = append(argv, string(currentArg))
	}

	return argv, nil
}

func main() {
	for {
		fmt.Fprint(os.Stdout, "$ ")

		inp, err := bufio.NewReader(os.Stdin).ReadString(delimiter)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading input:", err)
			os.Exit(1)
		}

		argv, err := ParseInput(inp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %s\n", err.Error())
			continue
		}
		// fmt.Printf("parsed %#v\n", argv)
		if len(argv) == 0 {
			continue
		}
		cmd := argv[0]

		switch cmd {
		case "exit":
			code := 0
			if len(argv) > 1 {
				code, err = strconv.Atoi(argv[1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "invalid arguments expected a number: %s", err.Error())
					continue
				}
			}
			os.Exit(code)

		case "echo":
			str := strings.Join(argv[1:], " ")
			fmt.Println(str)

		case "type":
			if len(argv) == 1 {
				continue
			}
			for i := 1; i < len(argv); i++ {
				arg := argv[i]
				if _, ok := builtinCmds[arg]; ok == true {
					fmt.Printf("%s is a shell builtin\n", arg)
					continue
				}

				if file, ok := findInPath(arg); ok == true {
					fmt.Printf("%s is %s\n", arg, file)
					continue
				}
				fmt.Printf("%s: not found\n", arg)
			}

		case "pwd":
			pwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				continue
			}
			fmt.Println(pwd)

		case "cd":
			if len(argv) == 1 {
				continue
			}
			if len(argv) > 2 {
				fmt.Fprintln(os.Stderr, "too many arguments")
			}
			dir := argv[1]
			if dir[0] == '~' {
				HOME := os.Getenv("HOME")
				dir = strings.Replace(dir, "~", HOME, 1)
			}
			if err := os.Chdir(dir); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: No such file or directory\n", cmd, dir)
				continue
			}

		default:
			if _, ok := findInPath(cmd); ok == true {
				cmd := exec.Command(cmd, argv[1:]...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err := cmd.Run()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error Executing: %v\n", err)
				}
				continue
			}
			fmt.Fprintf(os.Stderr, "%s: command not found\n", cmd)
		}
	}

}
