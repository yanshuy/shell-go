package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var delimiter byte = '\n'

func main() {
	cmds := map[string]struct{}{
		"exit": {},
		"echo": {},
		"type": {},
	}

	for {
		fmt.Fprint(os.Stdout, "$ ")

		inp, err := bufio.NewReader(os.Stdin).ReadString(delimiter)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading input:", err)
			os.Exit(1)
		}
		inpArr := strings.Split(strings.Trim(inp, string(delimiter)), " ")

		cmd := inpArr[0]
		switch cmd {
		case "exit":
			code, err := strconv.Atoi(inpArr[1])
			if err != nil {
				fmt.Println("invalid arguments expected a number")
				continue
			}
			os.Exit(code)
		case "echo":
			str := strings.Join(inpArr[1:], " ")
			fmt.Println(str)
		case "type":
			for i := 1; i < len(inpArr); i++ {
				arg := inpArr[i]
				if _, ok := cmds[arg]; ok != true {
					fmt.Printf("type: %s: not found\n", arg)
					continue
				}
				fmt.Printf("%s is a shell builtin\n", arg)
			}
		default:
			fmt.Println(cmd + ": command not found")
		}
	}

}
