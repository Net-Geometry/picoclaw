package commands

import (
	"context"
	"fmt"
)

func showCommand() Definition {
	return Definition{
		Name:        "show",
		Description: "Show current configuration",
		SubCommands: []SubCommand{
			{
				Name:        "model",
				Description: "Current model and provider",
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.GetModelInfo == nil {
						return req.Reply(unavailableMsg)
					}
					name, provider := rt.GetModelInfo()
					return req.Reply(fmt.Sprintf("Current Model: %s (Provider: %s)", name, provider))
				},
			},
			{
				Name:        "channel",
				Description: "Current channel",
				Handler: func(_ context.Context, req Request, _ *Runtime) error {
					return req.Reply(fmt.Sprintf("Current Channel: %s", req.Channel))
				},
			},
			{
				Name:        "agents",
				Description: "Registered agents",
				Handler:     agentsHandler(),
			},
			{
				Name:        "agent",
				Description: "Current manual agent override",
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.GetCurrentAgent == nil {
						return req.Reply(unavailableMsg)
					}
					current := rt.GetCurrentAgent()
					if current == "" {
						current = "auto"
					}
					return req.Reply(fmt.Sprintf("Current Agent: %s", current))
				},
			},
			{
				Name:        "project",
				Description: "Current manual project override",
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.GetCurrentProject == nil {
						return req.Reply(unavailableMsg)
					}
					current := rt.GetCurrentProject()
					if current == "" {
						current = "none"
					}
					return req.Reply(fmt.Sprintf("Current Project: %s", current))
				},
			},
		},
	}
}
