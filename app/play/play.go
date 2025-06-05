package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/term"
)

func main() {
	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		fmt.Println("Stdin is not a terminal.")
		os.Exit(1)
	}

	prevState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Println("Error setting raw mode:", err)
		os.Exit(1)
	}
	defer term.Restore(int(os.Stdin.Fd()), prevState)

	t := term.NewTerminal(os.Stdin, "$ ")
	for {
		line, err := t.ReadLine()
		if err != nil {
			fmt.Printf("Error reading line: %v\n", err)
			return
		}
		t.Write([]byte(line + "\n"))
	}
}

func readInput() (string, error) {
	fd := int(os.Stdin.Fd())

	prevState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Println("Error setting raw mode:", err)
		os.Exit(0)
	}
	defer term.Restore(int(os.Stdin.Fd()), prevState)

	reader := bufio.NewReader(os.Stdin)
	if !term.IsTerminal(fd) {
		reader := bufio.NewReader(os.Stdin)
		return reader.ReadString('\n')
	}

	var buf []rune
	pos := 0

	fmt.Fprint(os.Stdout, "$ ")

	for {
		r, _, err := reader.ReadRune()
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}

		switch r {
		case '\n', '\r':
			return string(buf), nil

		case 127: //backspace
			if pos > 0 {
				buf = append(buf[:pos-1], buf[pos:]...)
				pos--
			}

		case 3: // ctrl + c
			term.Restore(int(os.Stdin.Fd()), prevState)
			os.Exit(130)

		case 27: // escape
			r2, _, err := reader.ReadRune()
			if err != nil && err != io.EOF {
				log.Fatal(err)
			}
			if r2 == '[' {
				r3, _, err := reader.ReadRune()
				if err != nil && err != io.EOF {
					log.Fatal(err)
				}
				if r3 == 'D' && pos > 0 { // Left arrow
					pos--
				} else if r3 == 'C' && pos < len(buf) { // Right arrow
					pos++
				}
			}

		default:
			right := append([]rune{r}, buf[pos:]...)
			buf = append(buf[:pos], right...)
			pos++
		}

		fmt.Print("\r\033[K")
		fmt.Print("$ " + string(buf))
		if pos < len(buf) {
			fmt.Printf("\033[%dD", len(buf)-pos)
		}
	}
}
