package manus

import (
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
)

func init() {
	channels.RegisterSafeFactory(
		config.ChannelManus,
		func(bc *config.Channel, c *config.ManusSettings, b *bus.MessageBus) (channels.Channel, error) {
			return NewManusChannel(bc, c, b)
		},
	)
}
