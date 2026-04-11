package commands

import (
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /gif command
func (h *SlashCommandsHandler) gifCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeDeferredCreateMessage,
	})
	chain, err := h.ChainsService.GetChain(i.GuildID().String())
	if err != nil {
		return
	}
	gif, err := chain.MediaStore.GetMedia("gif")
	if err != nil || gif == "" {
		gif = "No valid gif found."
	}
	s.Rest.UpdateInteractionResponse(s.ApplicationID, i.Token(), discord.MessageUpdate{
		Content: &gif,
	})
}

// implementation of /image command
func (h *SlashCommandsHandler) imageCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeDeferredCreateMessage,
	})
	chain, err := h.ChainsService.GetChain(i.GuildID().String())
	if err != nil {
		return
	}
	media, err := chain.MediaStore.GetMedia("image")
	if err != nil || media == "" {
		media = "No valid image found."
	}
	s.Rest.UpdateInteractionResponse(s.ApplicationID, i.Token(), discord.MessageUpdate{
		Content: &media,
	})
}

// implementation of /video command
func (h *SlashCommandsHandler) videoCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeDeferredCreateMessage,
	})
	chain, err := h.ChainsService.GetChain(i.GuildID().String())
	if err != nil {
		return
	}
	media, err := chain.MediaStore.GetMedia("video")
	if err != nil || media == "" {
		media = "No valid video found."
	}
	s.Rest.UpdateInteractionResponse(s.ApplicationID, i.Token(), discord.MessageUpdate{
		Content: &media,
	})
}
