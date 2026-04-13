package commands

import (
	"context"
	"fmt"
	"rolando/internal/config"
	"rolando/internal/logger"
	"rolando/internal/utils"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /analytics command
func (h *SlashCommandsHandler) analyticsCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	ctx := context.Background()

	chainConf, err := h.ChainsService.GetChainConf(ctx, i.GuildID().String())
	if err != nil {
		logger.Errorf("Failed to fetch chain document for guild %s: %v", i.GuildID, err)
		return
	}
	analytics, err := h.ChainsService.NewMarkovAnalyzer(chainConf).GetRawAnalytics(ctx)
	if err != nil {
		logger.Errorf("failed to analyze chain: %v", err)
		return
	}
	// Constructing the embed
	botUser, ok := s.Caches.SelfUser()
	if !ok {
		logger.Errorf("Could not fetch selfUser")
		return
	}
	botAvatarUrl := botUser.AvatarURL(discord.WithSize(256))
	embed := discord.Embed{
		Title:       "Analytics",
		Description: "**Complexity Score**: indicates how *smart* the bot is.\nA higher value means smarter",
		Color:       0xFFD700, // Gold color
		Fields: []discord.EmbedField{
			{
				Name:   "Complexity Score",
				Value:  fmt.Sprintf("```%d```", analytics.ComplexityScore),
				Inline: new(true),
			},
			{
				Name:   "Cohesion",
				Value:  fmt.Sprintf("```%d```", analytics.NGramSize),
				Inline: new(true),
			},
			{
				Name:   "Vocabulary",
				Value:  fmt.Sprintf("```%d words```", analytics.Words),
				Inline: new(true),
			},
			{
				Name:   "\t", // Empty field for spacing
				Value:  "\t",
				Inline: new(false),
			},
			{
				Name:   "Gifs",
				Value:  fmt.Sprintf("```%d```", analytics.Gifs),
				Inline: new(true),
			},
			{
				Name:   "Videos",
				Value:  fmt.Sprintf("```%d```", analytics.Videos),
				Inline: new(true),
			},
			{
				Name:   "Images",
				Value:  fmt.Sprintf("```%d```", analytics.Images),
				Inline: new(true),
			},
			{
				Name:   "\t", // Empty field for spacing
				Value:  "\t",
				Inline: new(false),
			},
			{
				Name:   "Processed Messages",
				Value:  fmt.Sprintf("```%d```", analytics.Messages),
				Inline: new(true),
			},
			{
				Name:   "Size",
				Value:  fmt.Sprintf("```%s / %s```", utils.FormatBytes(analytics.Size), utils.FormatBytes(uint64(chainConf.MaxSizeMb*1024*1024))),
				Inline: new(true),
			},
		},
		Footer: &discord.EmbedFooter{
			Text:    fmt.Sprintf("Version: %s", config.Version),
			IconURL: *botAvatarUrl,
		},
	}

	// Send the response with the embed
	err = s.Rest.CreateInteractionResponse(i.ID(), i.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.NewMessageCreate().WithEmbeds(embed),
	})
	if err != nil {
		logger.Errorf("Failed to send analytics embed: %v", err)
	}
}
