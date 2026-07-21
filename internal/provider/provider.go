package provider

import (
	"context"
	"errors"

	"github.com/soundadam/njuprobe/internal/model"
)

var ErrUnavailable = errors.New("measurement providers are not implemented in this foundation build")

type Request struct {
	Command  model.Command
	IPFamily string
	Label    *string
	Note     *string
}

type Runner interface {
	Run(context.Context, Request) (model.RunSummary, error)
}

type UnavailableRunner struct{}

func (UnavailableRunner) Run(context.Context, Request) (model.RunSummary, error) {
	return model.RunSummary{}, ErrUnavailable
}
