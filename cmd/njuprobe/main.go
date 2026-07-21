package main

import (
	"context"
	"fmt"
	"os"

	"github.com/soundadam/njuprobe/internal/buildinfo"
	"github.com/soundadam/njuprobe/internal/cli"
	"github.com/soundadam/njuprobe/internal/consent"
	"github.com/soundadam/njuprobe/internal/provider"
	"github.com/soundadam/njuprobe/internal/storage"
)

func main() {
	historyDir, err := storage.DefaultHistoryDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "njuprobe: determine history directory: %v\n", err)
		os.Exit(1)
	}

	consentPath, err := consent.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "njuprobe: determine consent path: %v\n", err)
		os.Exit(1)
	}

	app := &cli.App{
		In:        os.Stdin,
		Out:       os.Stdout,
		Err:       os.Stderr,
		StdinTTY:  isTerminal(os.Stdin),
		StdoutTTY: isTerminal(os.Stdout),
		Version:   buildinfo.Version,
		Runner:    provider.UnavailableRunner{},
		History:   storage.New(historyDir),
		Consent:   consent.New(consentPath),
	}

	os.Exit(app.Execute(context.Background(), os.Args[1:]))
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
