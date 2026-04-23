package commands

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"rolando/cmd/idiscord/helpers"
	"rolando/internal/logger"
	"rolando/internal/repositories"
	"rolando/internal/stt"
	"rolando/internal/tts"
	"rolando/internal/utils"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
)

// implementation of /vc join command
func (h *SlashCommandsHandler) vcJoinCommand(client *bot.Client, i *events.ApplicationCommandInteractionCreate) {
	err := i.DeferCreateMessage(true)
	if err != nil {
		logger.Errorf("Failed to defer interaction response: %v", err)
		return
	}

	guildID := *i.GuildID()
	userID := i.User().ID

	// step 1: get the user's voice state
	voiceState, ok := h.Client.Caches.VoiceState(guildID, userID)
	if !ok || voiceState.ChannelID == nil {
		content := "You must be in a voice channel to use this command."
		_, err := client.Rest.UpdateInteractionResponse(client.ApplicationID, i.Token(), discord.NewMessageUpdate().WithContent(content))
		if err != nil {
			logger.Errorf("Failed to update interaction response: %v", err)
		}
		return
	}

	go func() {
		vcCtx, _ := context.WithCancel(context.Background())

		// step 2: join the voice channel
		conn := h.Client.VoiceManager.CreateConn(guildID)
		err = conn.Open(vcCtx, *voiceState.ChannelID, false, false)
		if err != nil || conn.Gateway().Status() != voice.StatusReady {
			channel, _ := h.Client.Caches.Channel(*voiceState.ChannelID)
			var channelName string
			if channel != nil {
				channelName = channel.Name()
			} else {
				channelName = "unknown"
			}
			content := fmt.Sprintf("Failed to join the voice channel: %s", channelName)
			logger.Errorln(content, err)
			_, err := client.Rest.UpdateInteractionResponse(client.ApplicationID, i.Token(), discord.NewMessageUpdate().WithContent(content))
			if err != nil {
				logger.Errorf("Failed to update interaction response: %v", err)
			}
			return
		}

		voiceChannel, _ := h.Client.Caches.Channel(*voiceState.ChannelID)
		var channelName string
		if voiceChannel != nil {
			channelName = voiceChannel.Name()
		} else {
			channelName = "unknown"
		}

		// step 3: having joined the vc, respond to the interaction
		content := fmt.Sprintf("Joined the voice channel '%s', i'll speak sometimes", channelName)
		_, err = client.Rest.UpdateInteractionResponse(client.ApplicationID, i.Token(), discord.NewMessageUpdate().WithContent(content))
		if err != nil {
			logger.Errorf("Failed to update interaction response: %v", err)
		}

		// step 4: generate a static TTS audio and stream it to the vc
		chainDoc, err := h.ChainsService.GetChainConf(vcCtx, guildID.String())
		if err != nil {
			logger.Errorf("Failed to retrieve chain document: %v", err)
			return
		}
		provider, err := tts.GenerateTTSProvider("i am here", chainDoc.TTSLanguage)
		if err != nil {
			logger.Errorf("Failed to generate TTS provider: %v", err)
			conn.Close(vcCtx)
			return
		}
		if err := helpers.SendTTSToConn(vcCtx, conn, provider); err != nil {
			if !errors.Is(err, io.EOF) {
				logger.Errorf("Failed to stream audio in '%s' in '%s': %v", channelName, chainDoc.Name, err)
			}
		}

		// step 5: start listening in the vc
		listenVc(h, i, conn, vcCtx, voiceChannel, chainDoc)
	}()

}

func getVCUsers(h *SlashCommandsHandler, guildID snowflake.ID, channelID snowflake.ID) ([]discord.Member, error) {
	var users []discord.Member
	for vs := range h.Client.Caches.VoiceStates(guildID) {
		if vs.ChannelID != nil && *vs.ChannelID == channelID {
			member, ok := h.Client.Caches.Member(guildID, vs.UserID)
			if ok {
				users = append(users, member)
			}
		}
	}
	return users, nil
}

func listenVc(h *SlashCommandsHandler, event *events.ApplicationCommandInteractionCreate, conn voice.Conn, vcCtx context.Context, voiceChannel discord.Channel, chainConf *repositories.ChainConfig) {
	var ttsMutex sync.Mutex
	var doneOnce sync.Once
	done := make(chan struct{})
	triggerCleanup := func() {
		doneOnce.Do(func() {
			close(done)
		})
	}

	guildID := *event.GuildID()
	botUserID := h.Client.ID()
	var channelName string
	if voiceChannel != nil {
		channelName = voiceChannel.Name()
	} else {
		channelName = "unknown"
	}

	// Listen for voice state updates to detect when we should leave
	listener := bot.NewListenerFunc(
		func(e events.GuildVoiceStateUpdate) {
			if e.VoiceState.GuildID != guildID {
				return
			}

			if e.VoiceState.UserID == botUserID {
				logger.Infof("Left voice channel '%s' in '%s', initiating cleanup...", channelName, chainConf.Name)
				triggerCleanup()
				return
			}

			currentUsers, _ := getVCUsers(h, guildID, voiceChannel.ID())
			if len(currentUsers) < 1 { // All other users have left
				triggerCleanup()
			}
		},
	)
	h.Client.EventManager.AddEventListeners(listener)

	// use the 'done' channel to instruct other goroutines to stop *before* cleanup.
	defer func(updateListener bot.EventListener) {
		h.Client.RemoveEventListeners(updateListener)
		triggerCleanup() // safely closes the done channel exactly once
		conn.Close(vcCtx)
		stt.FreeRecognizer(chainConf.ID)
		logger.Infof("Cleanup complete for VC '%s' in '%s'", channelName, chainConf.Name)
	}(listener)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if conn.Gateway().Status() != voice.StatusReady {
					logger.Warnf("Voice connection no longer ready, initiating cleanup...")
					triggerCleanup()
					return
				}
			case <-done:
				return
			}
		}
	}()

	go func() {
		pcmChan := make(chan *helpers.PCMPacket, 10)
		receiver := helpers.NewVoskOpusReceiver(chainConf.ID, chainConf.TTSLanguage, pcmChan)

		// Register the opus frame receiver with the voice connection
		conn.SetOpusFrameReceiver(receiver)

		defer func() {
			receiver.Close()
		}()

		for {
			select {
			case packet, ok := <-pcmChan:
				if !ok || packet == nil {
					logger.Warnf("PCM channel closed. initiating cleanup...")
					triggerCleanup()
					return
				}

				if conn.Gateway().Status() != voice.StatusReady {
					logger.Warnf("Voice connection lost during PCM processing, initiating cleanup...")
					triggerCleanup()
					return
				}

				pcm := packet.Sequence
				var audioData bytes.Buffer
				binary.Write(&audioData, binary.LittleEndian, pcm)
				text, err := stt.SpeechToTextNative(&audioData, chainConf.TTSLanguage, chainConf.ID)
				if err != nil {
					logger.Errorf("Failed Speech to Text: %v", err)
					continue
				}

				random := utils.GetRandom(1, 1000)
				if text != "" {
					h.ChainsService.Train(vcCtx, guildID.String(), text, chainConf.NGramSize, chainConf.MaxSizeBytes(), chainConf.MarkovMaxBranches)
					if strings.Contains(text, "rolando") {
						random = 1
					}
				}
				if random != 1 {
					continue
				}
				go func() {
					ttsMutex.Lock()
					defer ttsMutex.Unlock()

					if conn.Gateway().Status() != voice.StatusReady {
						logger.Warnf("Voice connection not ready before streaming, skipping...")
						return
					}

					msg, err := h.ChainsService.GenerateFiltered(vcCtx, guildID.String(), 10)
					if err != nil {
						logger.Errorf("Failed to generate random text in '%s' in '%s': %v", channelName, chainConf.Name, err)
						return
					}
					if msg == "" {
						logger.Warnln("Generated empty msg in vc handler")
						return
					}
					provider, err := tts.GenerateTTSProvider(msg, chainConf.TTSLanguage)
					if err != nil {
						logger.Errorf("Failed to generate random TTS provider in '%s' in '%s': %v", channelName, chainConf.Name, err)
						return
					}
					if err := helpers.SendTTSToConn(vcCtx, conn, provider); err != nil {
						if !errors.Is(err, io.EOF) {
							logger.Errorf("Failed to stream random TTS audio in '%s' in '%s': %v", channelName, chainConf.Name, err)
							triggerCleanup()
						}
					}
				}()

			case <-done:
				logger.Info("Received shutdown signal in Audio Processing goroutine, exiting")
				return
			}
		}
	}()

	select {
	case <-done:
		// Received signal from the Audio Processor or the VoiceStateUpdate Handler.
		// go into the defer block for cleanup
		return
	case <-time.After(8 * time.Hour): // timeout after 8 hours
		logger.Infof("VC Timeout in '%s' in '%s', initiating cleanup...", channelName, chainConf.Name)
	}
}
