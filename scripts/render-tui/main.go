package main

import (
	"fmt"
	"os"

	"github.com/soundadam/soundprobe/internal/ui"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: render-tui setup|selector|progress")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "setup":
		fmt.Print(ui.PreviewSetupLanguage())
	case "selector":
		fmt.Print(ui.PreviewSelector())
	case "progress":
		fmt.Print(ui.PreviewProgress())
	default:
		fmt.Fprintln(os.Stderr, "usage: render-tui setup|selector|progress")
		os.Exit(2)
	}
}
