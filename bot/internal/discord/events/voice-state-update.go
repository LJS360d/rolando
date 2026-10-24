package events

import (
	"context"
	"errors"
	"rolando/internal/config"
	"rolando/internal/discord/helpers"
	"rolando/internal/logger"
	"rolando/internal/tts"
	"rolando/internal/utils"
	"time"

	"github.com/disgoorg/disgo/events"
)

// for random vc joins (uses vc join rate)

// handler for VOICE_STATE_UPDATE event. Runs on the shard's read loop and
// fires for every mute, deafen and stream toggle, so everything up to the
// dice roll is local; the rest runs in its own goroutine.
func (h *EventsHandler) onVoiceStateUpdate(e *events.GuildVoiceStateUpdate) {
	vs := e.VoiceState
	if vs.UserID == h.Client.ID() || vs.UserID == 0 {
		return
	}
	if vs.ChannelID == nil || vs.GuildDeaf || vs.GuildMute || vs.SelfDeaf || vs.SelfMute {
		// ignore users leaving, muting or deafening
		return
	}
	if old := e.OldVoiceState.ChannelID; old != nil && *old == *vs.ChannelID {
		// same channel: some other state toggled
		return
	}
	if helpers.InVoice(vs.GuildID) {
		// already in a vc within the guild
		return
	}
	guildID := vs.GuildID.String()
	chainDoc, err := h.ChainsService.Guilds.PeekConfig(guildID)
	if err != nil || chainDoc.VcJoinRate == 0 || !utils.RatedChoice(chainDoc.VcJoinRate) {
		return
	}
	channel, ok := h.Client.Caches.Channel(*vs.ChannelID)
	if !ok {
		logger.Errorf("Failed to fetch channel for voice state update event")
		return
	}
	member := e.Member
	// the gateway member carries no guild, and the paywall check filters entitlements by it
	member.GuildID = vs.GuildID

	go func() {
		if !GuildSubscriptionCheck(h.Client, member, chainDoc, config.VoiceChatFeaturesSKU) {
			return
		}
		time.Sleep(time.Duration(2 * time.Second))
		vcCtx, vcCancel := context.WithCancel(context.Background())
		defer vcCancel()

		// Create and open voice connection
		sess, err := helpers.Join(vcCtx, h.Client.VoiceManager, vs.GuildID, *vs.ChannelID)
		if errors.Is(err, helpers.ErrInVoice) {
			return
		}
		if err != nil {
			logger.Errorf("Failed to join voice channel '%s': %v", channel.Name(), err)
			return
		}
		defer sess.Leave()

		content, err := h.ChainsService.GenerateFiltered(vcCtx, guildID, 100)
		if err != nil {
			logger.Errorf("Failed to generate text: %v", err)
			return
		}
		provider, err := tts.GenerateTTSProvider(content, chainDoc.TTSLanguage)
		if err != nil {
			logger.Errorf("Failed to generate TTS provider: %v", err)
			return
		}

		if err := sess.Play(vcCtx, provider); err != nil {
			logger.Errorf("Failed to stream audio: %v", err)
		}

		logger.Infof("Randomly spoke in voice channel '%s' in '%s'", channel.Name(), chainDoc.Name)
	}()
}
