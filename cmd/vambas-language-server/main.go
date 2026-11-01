package main

import (
	"context"
	"fmt"
	"os"

	"sihlfall.ch/vambas-vai/lsp"
)

func main() {
	// Stdout belongs exclusively to JSON-RPC framing.
	if err := lsp.NewServer(os.Stdin, os.Stdout).Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
