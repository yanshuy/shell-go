package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {

	for {
		fmt.Fprint(os.Stdout, "$ ")
		inp, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "error reading input:", err)
			os.Exit(1)
		}
		inp = inp[:len(inp)-1]
		inpArr := strings.Split(inp, " ")
		cmd := inpArr[0]

		switch cmd {
		case "exit":
			code, err := strconv.Atoi(inpArr[1])
			if err != nil {
				fmt.Println("invalid arguments expected a number")
				continue
			}
			os.Exit(code)
		default:
			fmt.Println(cmd + ": command not found")
		}
	}

}
