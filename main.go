package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/netobserv/network-observability-cli/cmd"
)

var (
	BuildVersion string
	BuildDate    string
)

func main() {
	// Initial log message
	fmt.Printf("Starting %s:\n=====\nBuild Version: %s\nBuild Date: %s\n\n",
		filepath.Base(os.Args[0]), BuildVersion, BuildDate)

	err := cmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}
