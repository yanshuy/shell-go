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
		default:
			fmt.Println(cmd + ": command not found")
		}
	}

}
