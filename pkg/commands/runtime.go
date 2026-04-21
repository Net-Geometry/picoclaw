package commands

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/config"
)

// Runtime provides runtime dependencies to command handlers. It is constructed
// per-request by the agent loop so that per-request state (like session scope)
// can coexist with long-lived callbacks (like GetModelInfo).
type Runtime struct {
	Config             *config.Config
	GetModelInfo       func() (name, provider string)
	AskSideQuestion    func(ctx context.Context, question string) (string, error)
	ListAgentIDs       func() []string
	ListDefinitions    func() []Definition
	ListSkillNames     func() []string
	GetEnabledChannels func() []string
	GetActiveTurn      func() any // Returning any to avoid circular dependency with agent package
	GetCurrentAgent    func() string
	GetCurrentProject  func() string
	SwitchModel        func(value string) (oldModel string, err error)
	SwitchAgent        func(value string) (oldAgent string, err error)
	SwitchProject      func(value string) (oldProject string, err error)
	SwitchChannel      func(value string) error
	ClearHistory       func() error
	ReloadConfig       func() error
}
