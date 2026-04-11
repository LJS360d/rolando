package commands

import (
	"context"
	"rolando/cmd/idiscord/helpers"
	"rolando/internal/logger"
	"rolando/internal/tts"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// implementation of /vc leave command
func (h *SlashCommandsHandler) vcLeaveCommand(client *bot.Client, event *events.ApplicationCommandInteractionCreate) {
	guildID := *event.GuildID()

	conn := h.Client.VoiceManager.GetConn(guildID)
	if conn == nil {
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

	chainDoc, _ := h.ChainsService.GetChainDocument(guildID.String())
	provider, err := tts.GenerateTTSProvider("bye bye", chainDoc.TTSLanguage)
	if err != nil {
		logger.Errorf("Failed to generate TTS provider: %v", err)
		return
	}

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	if err := helpers.SendTTSToConn(ctx, conn, provider); err != nil {
		logger.Errorf("Failed to stream audio: %v", err)
	} else {
		logger.Infof("Spoke Bye Bye message in vc, leaving...")
	}

	conn.Close(ctx)
}
