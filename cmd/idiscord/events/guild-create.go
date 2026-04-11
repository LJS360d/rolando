package events

import (
	"fmt"
	"rolando/internal/logger"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// handler for GuildJoin event
func (h *EventsHandler) onGuildCreate(e *events.GuildJoin) {
	logger.Infof("Joined guild %s", e.Guild.Name)
	_, err := h.ChainsService.CreateChain(e.Guild.ID.String(), e.Guild.Name)
	if err != nil {
		logger.Errorf("Error creating chain: %s", err)
		return
	}
	if e.Guild.SystemChannelID != nil {
		_, err = h.Client.Rest.CreateMessage(*e.Guild.SystemChannelID, discord.NewMessageCreate().
			WithContent(
				fmt.Sprintf("Hello %s.\nperform the command `/train` to use all the server's messages as training data", e.Guild.Name),
			))
		if err != nil {
			logger.Errorf("Failed to send welcome message: %v", err)
		}
	}
	UpdatePresence(h.Client)
}
