package commands

import (
	"context"
	"rolando/internal/logger"
	"rolando/internal/utils"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /opinion command
func (h *SlashCommandsHandler) opinionCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	options := i.SlashCommandInteractionData().Options
	var about string
	for _, option := range options {
		if option.Name == "about" && option.Type == discord.ApplicationCommandOptionTypeString {
			about = option.String()
			break
		}
	}

	if about == "" {
		s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: discord.MessageCreate{
				Content: "You must provide a word as the seed.",
				Flags:   discord.MessageFlagEphemeral,
			},
		})
		return
	}

	if err := i.DeferCreateMessage(false); err != nil {
		logger.Errorf("Failed to defer interaction response: %v", err)
		return
	}
	// building a cold chain can take seconds; keep it off the shard read loop
	go func() {
		msg, err := h.ChainsService.GenerateFromSeed(context.Background(), i.GuildID().String(), about, utils.GetRandom(8, 40))
		if err != nil {
			logger.Errorf("Failed to generate opinion: %v", err)
			msg = "Failed to generate text."
		} else if msg == "" {
			msg = "I have no opinion about that."
		}
		if _, err := s.Rest.UpdateInteractionResponse(s.ApplicationID, i.Token(), discord.NewMessageUpdate().WithContent(msg)); err != nil {
			logger.Errorf("Failed to update interaction response: %v", err)
		}
	}()
}
