package main

import (
	"fmt"
	"os"

	"github.com/Wei-Shaw/sub2api/internal/researchgateway"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "offline research gateway accepts no options")
		os.Exit(2)
	}
	if err := researchgateway.RunOffline(); err != nil {
		fmt.Fprintln(os.Stderr, "offline research failed")
		os.Exit(1)
	}
}
