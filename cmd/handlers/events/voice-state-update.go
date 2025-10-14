package events

import (
	"rolando/internal/config"
	"rolando/internal/logger"
	"rolando/internal/tts"
	"rolando/internal/utils"
	"time"

	"github.com/bwmarrin/discordgo"
)

// for random vc joins (uses vc join rate)

// handler for VOICE_STATE_UPDATE event
func (h *EventsHandler) onVoiceStateUpdate(s *discordgo.Session, e *discordgo.Event) {
	voiceStateUpdate, ok := e.Struct.(*discordgo.VoiceStateUpdate)
	if !ok {
		return
	}
	if voiceStateUpdate.UserID == s.State.User.ID || voiceStateUpdate.UserID == "" {
		// ignore updates for self or empty user ID
		return
	}
	if voiceStateUpdate.ChannelID == "" || voiceStateUpdate.Deaf || voiceStateUpdate.Mute {
		// ignore without channel (e.g. user left voice channel)
		// ignore user deafening or muting
		//
		return
	}
	_, alreadyInUse := s.VoiceConnections[voiceStateUpdate.GuildID]
	if alreadyInUse {
		// stop if already in use in another vc within the guild
		return
	}
	channel, err := s.State.Channel(voiceStateUpdate.ChannelID)
	if err != nil {
		logger.Errorf("Failed to fetch channel for voice state update event: %v", err)
		return
	}
	chainDoc, err := h.ChainsService.GetChainDocument(voiceStateUpdate.GuildID)
	if err != nil {
		logger.Errorf("Failed to fetch chain document for guild %s: %v", voiceStateUpdate.GuildID, err)
		return
	}
	hasVcFeatures := GuildSubscriptionCheck(s, voiceStateUpdate.Member, chainDoc, config.VoiceChatFeaturesSKU)
	if !hasVcFeatures {
		return
	}
	if chainDoc.VcJoinRate == 0 {
		// never join for guilds that have it disabled
		return
	}
	if utils.GetRandom(1, chainDoc.VcJoinRate) != 1 {
		return
	}
	go func() {
		time.Sleep(time.Duration(2 * time.Second))
		vc, err := s.ChannelVoiceJoin(voiceStateUpdate.GuildID, voiceStateUpdate.ChannelID, false, false)
		if err != nil {
			logger.Errorf("Failed to join voice channel '%s': %v", channel.Name, err)
			return
		}
		chain, _ := h.ChainsService.GetChain(chainDoc.ID)
		d, err := tts.GenerateTTSDecoder(chain.TalkFiltered(100), chainDoc.TTSLanguage)
		if err != nil {
			logger.Errorf("Failed to generate TTS decoder: %v", err)
			return
		}
		if err := utils.StreamAudioDecoder(vc, d); err != nil {
			logger.Errorf("Failed to stream audio: %v", err)
		}
		err = vc.Disconnect()
		vc.Close()
		if err != nil {
			logger.Errorf("Failed to disconnect from voice channel '%s' in '%s': %v", channel.Name, chainDoc.Name, err)
		}
		vc.Close()
		logger.Infof("Randomly spoke in voice channel '%s' in '%s'", channel.Name, chainDoc.Name)
	}()
}
