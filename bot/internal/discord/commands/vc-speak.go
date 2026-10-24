package commands

import (
	"context"
	"errors"
	"io"
	"rolando/internal/discord/helpers"
	"rolando/internal/logger"
	"rolando/internal/tts"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
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

		// speak in the session already there, else join just for this
		sess := helpers.Current(guildID)
		if sess == nil {
			_, err := client.Rest.UpdateInteractionResponse(appID, token, discord.NewMessageUpdate().WithContent("Joining Voice Channel..."))
			if err != nil {
				logger.Errorf("Failed to update interaction response: %v", err)
			}

			sess, err = helpers.Join(vcCtx, h.Client.VoiceManager, guildID, channelID)
			if err != nil {
				content := "Failed to join the voice channel: " + err.Error()
				client.Rest.UpdateInteractionResponse(appID, token, discord.NewMessageUpdate().WithContent(content))
				return
			}
			defer sess.Leave()
		}

		chainDoc, err := h.ChainsService.GetChainConf(vcCtx, guildID.String())
		if err != nil {
			client.Rest.UpdateInteractionResponse(appID, token, discord.NewMessageUpdate().WithContent("Failed to retrieve chain data."))
			return
		}

		content, err := h.ChainsService.GenerateFiltered(vcCtx, guildID.String(), 100)
		if err != nil {
			logger.Errorf("Failed to generate text: %v", err)
			return
		}
		_, err = client.Rest.UpdateInteractionResponse(appID, token, discord.NewMessageUpdate().WithContent(content))
		if err != nil {
			logger.Errorf("Failed to update interaction response: %v", err)
		}

		provider, err := tts.GenerateTTSProvider(content, chainDoc.TTSLanguage)
		if err != nil {
			logger.Errorf("Failed to generate TTS provider: %v", err)
			return
		}

		if err := sess.Play(vcCtx, provider); err != nil {
			if !errors.Is(err, io.EOF) {
				logger.Errorf("Failed to stream audio: %v", err)
			}
		}
	}()
}
