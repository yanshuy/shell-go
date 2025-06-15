package main

import (
	"log"
)

func main() {
	s, err := NewShell()
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	err = s.Run()
	if err != nil {
		log.Fatal(err)
	}
}
