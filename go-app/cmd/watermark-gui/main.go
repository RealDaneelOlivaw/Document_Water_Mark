package main

import (
	"log"

	"pptwatermark/goapp/internal/gui"
)

func main() {
	if err := gui.Run(); err != nil {
		log.Fatal(err)
	}
}
