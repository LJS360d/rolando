package events

import (
	"context"
	"rolando/cmd/idiscord/helpers"
	"rolando/internal/config"
	"rolando/internal/logger"
	"rolando/internal/tts"
	"rolando/internal/utils"
	"time"

	"github.com/disgoorg/disgo/events"
)

// for random vc joins (uses vc join rate)

// handler for VOICE_STATE_UPDATE event
func (h *EventsHandler) onVoiceStateUpdate(e *events.GuildVoiceStateUpdate) {
	ctx := context.Background()

	// Convert snowflake IDs to strings for internal use
	userID := e.VoiceState.UserID.String()
	guildID := e.VoiceState.GuildID.String()
	botID := h.Client.ID().String()

	if userID == botID || userID == "" {
		// ignore updates for self or empty user ID
		return
	}
	if e.VoiceState.ChannelID == nil || e.VoiceState.GuildDeaf || e.VoiceState.GuildMute || e.VoiceState.SelfDeaf || e.VoiceState.SelfMute {
		// ignore without channel (user left voice channel)
		// ignore user deafening or muting
		return
	}

	// Check if already connected in this guild
	conn := h.Client.VoiceManager.GetConn(e.VoiceState.GuildID)
	if conn != nil {
		logger.Warnf("User %s tried to join voice channel %s, but bot is already connected in guild %s", userID, e.VoiceState.ChannelID, guildID)
		// stop if already in use in another vc within the guild
		return
	}

	// Get channel information
	channel, ok := h.Client.Caches.Channel(*e.VoiceState.ChannelID)
	if !ok {
		logger.Errorf("Failed to fetch channel for voice state update event")
		return
	}

	chainDoc, err := h.ChainsService.GetChainConf(ctx, guildID)
	if err != nil {
		logger.Errorf("Failed to fetch chain document for guild %s: %v", guildID, err)
		return
	}

	// Get member for subscription check
	member, err := h.Client.Rest.GetMember(e.VoiceState.GuildID, e.VoiceState.UserID)
	if err != nil {
		logger.Errorf("Failed to fetch member for voice state update: %v", err)
		return
	}
	if member == nil {
		logger.Warnf("Failed to fetch member for voice state update: member is nil")
		return
	}

	hasVcFeatures := GuildSubscriptionCheck(h.Client, *member, chainDoc, config.VoiceChatFeaturesSKU)
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
		vcCtx, vcCancel := context.WithCancel(context.Background())
		defer vcCancel()

		// Create and open voice connection
		conn := h.Client.VoiceManager.CreateConn(e.VoiceState.GuildID)
		err := conn.Open(vcCtx, *e.VoiceState.ChannelID, false, false)
		if err != nil {
			logger.Errorf("Failed to join voice channel '%s': %v", channel.Name(), err)
			return
		}

		content, err := h.ChainsService.RedisRepo.GenerateFiltered(vcCtx, guildID, 100, chainDoc.NGramSize)
		if err != nil {
			logger.Errorf("Failed to generate text: %v", err)
			return
		}
		provider, err := tts.GenerateTTSProvider(content, chainDoc.TTSLanguage)
		if err != nil {
			logger.Errorf("Failed to generate TTS provider: %v", err)
			conn.Close(vcCtx)
			return
		}

		if err := helpers.SendTTSToConn(vcCtx, conn, provider); err != nil {
			logger.Errorf("Failed to stream audio: %v", err)
		}

		conn.Close(vcCtx)

		logger.Infof("Randomly spoke in voice channel '%s' in '%s'", channel.Name(), chainDoc.Name)
	}()
}
