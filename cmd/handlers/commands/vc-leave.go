package commands

import (
	"rolando/internal/logger"
	"rolando/internal/tts"
	"rolando/internal/utils"

	"github.com/bwmarrin/discordgo"
)

// implementation of /vc leave command
func (h *SlashCommandsHandler) vcLeaveCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	vc, exists := s.VoiceConnections[i.GuildID]
	if !exists {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "I am not connected to a voice channel.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "I am leaving the voice channel",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	chainDoc, _ := h.ChainsService.GetChainDocument(i.GuildID)
	d, err := tts.GenerateTTSDecoder("bye bye", chainDoc.TTSLanguage)
	if err != nil {
		logger.Errorf("Failed to generate TTS decoder: %v", err)
		return
	}
	if err := utils.StreamAudioDecoder(vc, d); err != nil {
		logger.Errorf("Failed to stream audio: %v", err)
	} else {
		logger.Infof("Spoke Bye Bye message in vc, leaving...")
	}
	err = vc.Disconnect()
	if err != nil {
		logger.Errorf("Failed to disconnect from voice channel: %v", err)
	}
}
