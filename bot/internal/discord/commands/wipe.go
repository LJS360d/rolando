package commands

import (
	"context"
	"fmt"
	"rolando/internal/logger"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /wipe command
func (h *SlashCommandsHandler) wipeCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	options := i.SlashCommandInteractionData().Options
	var data string
	for _, option := range options {
		if option.Name == "data" && option.Type == discord.ApplicationCommandOptionTypeString {
			data = option.String()
			break
		}
	}

	if data == "" {
		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "You must provide the data to be erased.",
				Flags:   discord.MessageFlagEphemeral,
			},
		})
		return
	}

	if err := i.DeferCreateMessage(false); err != nil {
		logger.Errorf("Failed to defer interaction response: %v", err)
		return
	}
	go func() {
		content := fmt.Sprintf("Deleted `%s`", data)
		// 2k is the max msg length, give quite some headroom
		if len(content) > 1500 {
			// TODO truncate to the first 1.5k regardless
			content = "Deleted the specified data."
		}
		if err := h.ChainsService.DeleteTextData(context.Background(), i.GuildID().String(), data); err != nil {
			logger.Errorf("Failed to delete text data: %v", err)
			content = "Failed to delete the specified data."
		}
		if _, err := s.Rest.UpdateInteractionResponse(s.ApplicationID, i.Token(), discord.NewMessageUpdate().WithContent(content)); err != nil {
			logger.Errorf("Failed to update interaction response: %v", err)
		}
	}()
}
