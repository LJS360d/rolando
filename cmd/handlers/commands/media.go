package commands

import "github.com/bwmarrin/discordgo"

// implementation of /gif command
func (h *SlashCommandsHandler) gifCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	chain, err := h.ChainsService.GetChain(i.GuildID)
	if err != nil {
		return
	}
	gif, err := chain.MediaStorage.GetMedia("gif")
	if err != nil || gif == "" {
		gif = "No valid gif found."
	}
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &gif,
	})
}

// implementation of /image command
func (h *SlashCommandsHandler) imageCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	chain, err := h.ChainsService.GetChain(i.GuildID)
	if err != nil {
		return
	}
	image, err := chain.MediaStorage.GetMedia("image")
	if err != nil || image == "" {
		image = "No valid image found."
	}
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &image,
	})
}

// implementation of /video command
func (h *SlashCommandsHandler) videoCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	chain, err := h.ChainsService.GetChain(i.GuildID)
	if err != nil {
		return
	}
	video, err := chain.MediaStorage.GetMedia("video")
	if err != nil || video == "" {
		video = "No valid video found."
	}
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &video,
	})
}
