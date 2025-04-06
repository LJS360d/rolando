package commands

import (
	"fmt"
	"rolando/internal/config"
	"rolando/internal/logger"
	"rolando/internal/model"
	"rolando/internal/utils"

	"github.com/bwmarrin/discordgo"
)

// implementation of /analytics command
func (h *SlashCommandsHandler) analyticsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Fetch the chain data for the given guild
	chain, err := h.ChainsService.GetChain(i.GuildID)
	if err != nil {
		logger.Errorf("Failed to fetch chain for guild %s: %v", i.GuildID, err)
		return
	}
	chainDoc, err := h.ChainsService.GetChainDocument(i.GuildID)
	if err != nil {
		logger.Errorf("Failed to fetch chain document for guild %s: %v", i.GuildID, err)
		return
	}
	analytics := model.NewMarkovChainAnalyzer(chain).GetRawAnalytics()
	// Constructing the embed
	embed := &discordgo.MessageEmbed{
		Title:       "Analytics",
		Description: "**Complexity Score**: indicates how *smart* the bot is.\nA higher value means smarter",
		Color:       0xFFD700, // Gold color
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Complexity Score",
				Value:  fmt.Sprintf("```%d```", analytics.ComplexityScore),
				Inline: true,
			},
			{
				Name:   "Vocabulary",
				Value:  fmt.Sprintf("```%d words```", analytics.Words),
				Inline: true,
			},
			{
				Name:   "\t", // Empty field for spacing
				Value:  "\t",
				Inline: false,
			},
			{
				Name:   "Gifs",
				Value:  fmt.Sprintf("```%d```", analytics.Gifs),
				Inline: true,
			},
			{
				Name:   "Videos",
				Value:  fmt.Sprintf("```%d```", analytics.Videos),
				Inline: true,
			},
			{
				Name:   "Images",
				Value:  fmt.Sprintf("```%d```", analytics.Images),
				Inline: true,
			},
			{
				Name:   "\t", // Empty field for spacing
				Value:  "\t",
				Inline: false,
			},
			{
				Name:   "Processed Messages",
				Value:  fmt.Sprintf("```%d```", analytics.Messages),
				Inline: true,
			},
			{
				Name:   "Size",
				Value:  fmt.Sprintf("```%s / %s```", utils.FormatBytes(analytics.Size), utils.FormatBytes(uint64(chainDoc.MaxSizeMb*1024*1024))),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    fmt.Sprintf("Version: %s", config.Version),
			IconURL: s.State.User.AvatarURL("256"),
		},
	}

	// Send the response with the embed
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		logger.Errorf("Failed to send analytics embed: %v", err)
	}
}
