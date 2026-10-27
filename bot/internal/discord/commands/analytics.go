package commands

import (
	"context"
	"fmt"
	ianalytics "rolando/internal/analytics"
	"rolando/internal/config"
	"rolando/internal/logger"
	"rolando/internal/utils"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /analytics command
func (h *SlashCommandsHandler) analyticsCommand(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	if err := i.DeferCreateMessage(false); err != nil {
		logger.Errorf("Failed to defer interaction response: %v", err)
		return
	}
	go h.analytics(s, i)
}

func (h *SlashCommandsHandler) analytics(s *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	fail := func(format string, args ...any) {
		logger.Errorf(format, args...)
		s.Rest.UpdateInteractionResponse(s.ApplicationID, i.Token(), discord.NewMessageUpdate().WithContent("Failed to compute analytics."))
	}
	if _, err := h.ChainsService.GetChainConf(context.Background(), i.GuildID().String()); err != nil {
		fail("Failed to fetch chain document for guild %s: %v", i.GuildID(), err)
		return
	}
	summary, err := h.ChainsService.Summary(i.GuildID().String())
	if err != nil {
		fail("failed to analyze chain: %v", err)
		return
	}
	analytics := ianalytics.FromSummary(summary)
	botUser, ok := s.Caches.SelfUser()
	if !ok {
		fail("Could not fetch selfUser")
		return
	}
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
				Value:  fmt.Sprintf("```%s / %s```", utils.FormatBytes(analytics.Size), utils.FormatBytes(summary.Config.MaxSizeBytes())),
				Inline: new(true),
			},
		},
		Footer: &discord.EmbedFooter{
			Text:    fmt.Sprintf("Version: %s", config.Version),
			IconURL: botUser.EffectiveAvatarURL(discord.WithSize(256)),
		},
	}

	_, err = s.Rest.UpdateInteractionResponse(s.ApplicationID, i.Token(), discord.NewMessageUpdate().WithEmbeds(embed))
	if err != nil {
		logger.Errorf("Failed to send analytics embed: %v", err)
	}
}
