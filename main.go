package main

import (
	"log"

	"github.com/netobserv/network-observability-cli/cmd"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}
