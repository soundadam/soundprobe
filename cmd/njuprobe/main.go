package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/soundadam/njuprobe/internal/buildinfo"
	"github.com/soundadam/njuprobe/internal/cli"
	"github.com/soundadam/njuprobe/internal/consent"
	"github.com/soundadam/njuprobe/internal/helper"
	"github.com/soundadam/njuprobe/internal/model"
	"github.com/soundadam/njuprobe/internal/network"
	"github.com/soundadam/njuprobe/internal/provider"
	"github.com/soundadam/njuprobe/internal/provider/campus"
	"github.com/soundadam/njuprobe/internal/provider/mlab"
	"github.com/soundadam/njuprobe/internal/storage"
	"github.com/soundadam/njuprobe/internal/target"
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

	helperResolver := helper.NewResolver()
	providers := map[model.Provider]provider.MeasurementProvider{}
	for _, station := range target.Stations() {
		if !station.TerminalSupported {
			continue
		}
		for _, spec := range []*target.Spec{station.IPv4, station.IPv6} {
			if spec == nil {
				continue
			}
			providers[spec.Provider] = campus.NewTarget(helperResolver, campus.Config{
				Provider:   spec.Provider,
				Label:      spec.Label,
				Family:     spec.Family,
				ServerName: spec.ServerName,
				ServerURL:  spec.ServerURL,
			})
		}
	}
	mlabRunner := mlab.New(helperResolver)
	providers[model.ProviderMLab] = mlabRunner
	measurementRunner := provider.SummaryRunner{
		ToolVersion: buildinfo.Version,
		Campus:      campus.New(helperResolver),
		MLab:        mlabRunner,
		Providers:   providers,
		Snapshot:    network.Snapshot,
	}

	app := &cli.App{
		In:        os.Stdin,
		Out:       os.Stdout,
		Err:       os.Stderr,
		StdinTTY:  isTerminal(os.Stdin),
		StdoutTTY: isTerminal(os.Stdout),
		Version:   buildinfo.Version,
		Runner:    measurementRunner,
		History:   storage.New(historyDir),
		Consent:   consent.New(consentPath),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	exitCode := app.Execute(ctx, os.Args[1:])
	stop()
	os.Exit(exitCode)
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
