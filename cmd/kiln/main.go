package main

import (
	"fmt"
	"os"

	"github.com/fisayoafolayan/kiln/internal/cmd"
)

func main() {
	if err := cmd.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
