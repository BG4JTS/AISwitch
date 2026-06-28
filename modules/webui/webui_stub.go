//go:build !webui
// +build !webui

package webui

import (
	"context"

	"github.com/yourusername/ais/core"
	"github.com/yourusername/ais/pkg/module"
)

type stub struct{}

func init() {
	core.RegisterModule(&stub{})
}

func (s *stub) Name() string     { return "webui" }
func (s *stub) Requires() []string { return nil }
func (s *stub) Enabled() bool    { return false }
func (s *stub) Init(_ *module.CoreContext) error { return nil }
func (s *stub) Start(_ context.Context) error    { return nil }
func (s *stub) Stop() error                      { return nil }
