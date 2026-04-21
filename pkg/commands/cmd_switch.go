package commands

import (
	"context"
	"fmt"
	"strings"
)

func switchCommand() Definition {
	return Definition{
		Name:        "switch",
		Description: "Switch model",
		SubCommands: []SubCommand{
			{
				Name:        "model",
				Description: "Switch to a different model",
				ArgsUsage:   "to <name>",
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.SwitchModel == nil {
						return req.Reply(unavailableMsg)
					}
					// Parse: /switch model to <value>
					value := nthToken(req.Text, 3) // tokens: [/switch, model, to, <value>]
					if nthToken(req.Text, 2) != "to" || value == "" {
						return req.Reply("Usage: /switch model to <name>")
					}
					oldModel, err := rt.SwitchModel(value)
					if err != nil {
						return req.Reply(err.Error())
					}
					return req.Reply(fmt.Sprintf("Switched model from %s to %s", oldModel, value))
				},
			},
			{
				Name:        "agent",
				Description: "Switch to a different active agent",
				ArgsUsage:   "to <id>",
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.SwitchAgent == nil {
						return req.Reply(unavailableMsg)
					}
					value := nthToken(req.Text, 3) // tokens: [/switch, agent, to, <value>]
					if nthToken(req.Text, 2) != "to" || value == "" {
						return req.Reply("Usage: /switch agent to <id|none>")
					}
					oldAgent, err := rt.SwitchAgent(value)
					if err != nil {
						return req.Reply(err.Error())
					}
					if oldAgent == "" {
						oldAgent = "auto"
					}
					if strings.EqualFold(value, "none") || strings.EqualFold(value, "clear") || strings.EqualFold(value, "off") {
						return req.Reply(fmt.Sprintf("Cleared manual agent override (was %s)", oldAgent))
					}
					return req.Reply(fmt.Sprintf("Switched agent from %s to %s", oldAgent, value))
				},
			},
			{
				Name:        "project",
				Description: "Switch active project context",
				ArgsUsage:   "to <name|none>",
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.SwitchProject == nil {
						return req.Reply(unavailableMsg)
					}
					value := nthToken(req.Text, 3) // tokens: [/switch, project, to, <value>]
					if nthToken(req.Text, 2) != "to" || value == "" {
						return req.Reply("Usage: /switch project to <name|none>")
					}
					oldProject, err := rt.SwitchProject(value)
					if err != nil {
						return req.Reply(err.Error())
					}
					if oldProject == "" {
						oldProject = "none"
					}
					return req.Reply(fmt.Sprintf("Switched project from %s to %s", oldProject, value))
				},
			},
			{
				Name:        "channel",
				Description: "Moved to /check channel",
				Handler: func(_ context.Context, req Request, _ *Runtime) error {
					return req.Reply("This command has moved. Please use: /check channel <name>")
				},
			},
		},
	}
}
