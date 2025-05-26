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
}

func main() {
	for {
		fmt.Fprint(os.Stdout, "$ ")

		inp, err := bufio.NewReader(os.Stdin).ReadString(delimiter)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading input:", err)
			os.Exit(1)
		}
		argv := strings.Fields(strings.Trim(inp, string(delimiter)))
		cmd := argv[0]

		switch cmd {
		case "exit":
			code := 0
			if len(argv) > 1 {
				code, err = strconv.Atoi(argv[1])
				if err != nil {
					fmt.Println("invalid arguments expected a number")
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

		default:
			if _, ok := findInPath(cmd); ok == true {
				cmd := exec.Command(cmd, argv[1:]...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Run()
				continue
			}
			fmt.Println(cmd + ": command not found")
		}
	}

}
