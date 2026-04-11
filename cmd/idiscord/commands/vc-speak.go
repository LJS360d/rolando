package commands

import (
	"context"
	"errors"
	"io"
	"rolando/cmd/idiscord/helpers"
	"rolando/internal/logger"
	"rolando/internal/tts"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/voice"
)

// implementation of /vc speak command
func (h *SlashCommandsHandler) vcSpeakCommand(client *bot.Client, event *events.ApplicationCommandInteractionCreate) {
	guildID := *event.GuildID()
	userID := event.User().ID

	// step 1: get the user's voice state
	voiceState, ok := h.Client.Caches.VoiceState(guildID, userID)
	if !ok || voiceState.ChannelID == nil {
		err := event.CreateMessage(discord.NewMessageCreate().
			WithContent("You must be in a voice channel to use this command.").
			WithEphemeral(true))
		if err != nil {
			logger.Errorf("Failed to send interaction response: %v", err)
		}
		return
	}

	err := event.DeferCreateMessage(false)
	if err != nil {
		logger.Errorf("Failed to defer interaction response: %v", err)
		return
	}

	channelID := *voiceState.ChannelID
	token := event.Token()
	appID := client.ApplicationID
	go func() {
		vcCtx, vcCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer vcCancel()

		var conn voice.Conn
		existingConn := h.Client.VoiceManager.GetConn(guildID)
		if existingConn == nil {
			_, err := client.Rest.UpdateInteractionResponse(appID, token, discord.NewMessageUpdate().WithContent("Joining Voice Channel..."))
			if err != nil {
				logger.Errorf("Failed to update interaction response: %v", err)
			}

			conn = h.Client.VoiceManager.CreateConn(guildID)
			err = conn.Open(vcCtx, channelID, false, false)
			if err != nil {
				content := "Failed to join the voice channel: " + err.Error()
				client.Rest.UpdateInteractionResponse(appID, token, discord.NewMessageUpdate().WithContent(content))
				return
			}

			if conn.Gateway().Status() != voice.StatusReady {
				logger.Warnf("Connection status is %v, attempting to proceed anyway...", conn.Gateway().Status())
			}
		} else {
			conn = existingConn
		}

		chainDoc, err := h.ChainsService.GetChainDocument(guildID.String())
		if err != nil {
			client.Rest.UpdateInteractionResponse(appID, token, discord.NewMessageUpdate().WithContent("Failed to retrieve chain data."))
			return
		}

		chain, _ := h.ChainsService.GetChain(chainDoc.ID)
		content := chain.TalkFiltered(100)
		_, err = client.Rest.UpdateInteractionResponse(appID, token, discord.NewMessageUpdate().WithContent(content))
		if err != nil {
			logger.Errorf("Failed to update interaction response: %v", err)
		}

		provider, err := tts.GenerateTTSProvider(content, chainDoc.TTSLanguage)
		if err != nil {
			logger.Errorf("Failed to generate TTS provider: %v", err)
			return
		}

		// This will now block correctly without racing the internal reader
		if err := helpers.SendTTSToConn(vcCtx, conn, provider); err != nil {
			if !errors.Is(err, io.EOF) {
				logger.Errorf("Failed to stream audio: %v", err)
			}
		}
		conn.Close(vcCtx)
	}()
}
