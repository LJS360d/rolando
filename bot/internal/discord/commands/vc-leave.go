package commands

import (
	"context"
	"rolando/internal/discord/helpers"
	"rolando/internal/logger"
	"rolando/internal/tts"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /vc leave command
func (h *SlashCommandsHandler) vcLeaveCommand(client *bot.Client, event *events.ApplicationCommandInteractionCreate) {
	ctx := context.Background()
	guildID := *event.GuildID()

	// Current would wait out a join in progress, and the join needs this
	// read loop to finish
	if !helpers.InVoice(guildID) {
		err := event.CreateMessage(discord.NewMessageCreate().
			WithContent("I am not connected to a voice channel.").
			WithEphemeral(true))
		if err != nil {
			logger.Errorf("Failed to send interaction response: %v", err)
		}
		return
	}

	err := event.CreateMessage(discord.NewMessageCreate().
		WithContent("I am leaving the voice channel").
		WithEphemeral(true))
	if err != nil {
		logger.Errorf("Failed to send interaction response: %v", err)
		return
	}

	// the goodbye plays for up to 5s; keep it off the shard read loop
	go func() {
		sess := helpers.Current(guildID)
		if sess == nil {
			return
		}
		defer sess.Leave()
		chainDoc, err := h.ChainsService.GetChainConf(ctx, guildID.String())
		if err != nil {
			logger.Errorf("Failed to retrieve chain document: %v", err)
			return
		}
		provider, err := tts.PhraseProvider("bye bye", chainDoc.TTSLanguage)
		if err != nil {
			logger.Errorf("Failed to generate TTS provider: %v", err)
			return
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := sess.Play(ctx, provider); err != nil {
			logger.Errorf("Failed to stream audio: %v", err)
		} else {
			logger.Infof("Spoke Bye Bye message in vc, leaving...")
		}
	}()
}
