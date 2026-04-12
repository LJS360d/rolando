package commands

import (
	"context"
	"fmt"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /gif command
func (h *SlashCommandsHandler) gifCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	h.mediaCommand(s, i, "gif")
}

// implementation of /image command
func (h *SlashCommandsHandler) imageCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	h.mediaCommand(s, i, "image")
}

// implementation of /video command
func (h *SlashCommandsHandler) videoCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	h.mediaCommand(s, i, "video")
}

// common helper
func (h *SlashCommandsHandler) mediaCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate, kind string) {
	s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeDeferredCreateMessage,
	})
	media, err := h.ChainsService.RedisRepo.GetRandomMedia(context.Background(), i.GuildID().String(), kind)
	if err != nil || media == "" {
		media = fmt.Sprintf("No valid %s found.", kind)
	}
	s.Rest.UpdateInteractionResponse(s.ApplicationID, i.Token(), discord.MessageUpdate{
		Content: &media,
	})
}
