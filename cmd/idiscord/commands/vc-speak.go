package commands

import (
	"rolando/cmd/idiscord/helpers"
	"rolando/internal/logger"
	"rolando/internal/tts"

	"github.com/bwmarrin/discordgo"
)

// implementation of /vc speak command
func (h *SlashCommandsHandler) vcSpeakCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// step 1: get the user's voice state
	voiceState, err := s.State.VoiceState(i.GuildID, i.Member.User.ID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You must be in a voice channel to use this command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	var vc *discordgo.VoiceConnection
	vc, exists := s.VoiceConnections[voiceState.GuildID]
	if !exists {
		content := "Joining Voice Channel..."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		// join the voice channel
		vc, err = s.ChannelVoiceJoin(i.GuildID, voiceState.ChannelID, false, false)
		if err != nil || !vc.Ready {
			content := "You must be in a voice channel to use this command."
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &content,
			})
			return
		}
	}

	chainDoc, err := h.ChainsService.GetChainDocument(voiceState.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to retrieve chain data.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	chain, _ := h.ChainsService.GetChain(chainDoc.ID)
	content := chain.TalkFiltered(100)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
	d, err := tts.GenerateTTSDecoder(content, chainDoc.TTSLanguage)
	if err != nil {
		logger.Errorf("Failed to generate TTS decoder: %v", err)
		return
	}
	if err := helpers.StreamAudioDecoder(vc, d); err != nil {
		logger.Errorf("Failed to stream audio: %v", err)
	}
	err = vc.Disconnect()
	if err != nil {
		logger.Errorf("Failed to disconnect from voice channel: %v", err)
	}
	vc.Close()
}
