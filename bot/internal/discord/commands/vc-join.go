package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"rolando/internal/discord/helpers"
	"rolando/internal/guilds"
	"rolando/internal/logger"
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
		vcCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// step 2: join the voice channel
		sess, err := helpers.Join(vcCtx, h.Client.VoiceManager, guildID, *voiceState.ChannelID)
		if err != nil {
			channel, _ := h.Client.Caches.Channel(*voiceState.ChannelID)
			var channelName string
			if channel != nil {
				channelName = channel.Name()
			} else {
				channelName = "unknown"
			}
			content := fmt.Sprintf("Failed to join the voice channel: %s", channelName)
			if errors.Is(err, helpers.ErrInVoice) {
				content = "I am already in a voice channel in this server."
			}
			logger.Errorf("%s %v", content, err)
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
			sess.Leave()
			return
		}
		provider, err := tts.PhraseProvider("i am here", chainDoc.TTSLanguage)
		if err != nil {
			logger.Errorf("Failed to generate TTS provider: %v", err)
			sess.Leave()
			return
		}
		if err := sess.Play(vcCtx, provider); err != nil {
			if !errors.Is(err, io.EOF) {
				logger.Errorf("Failed to stream audio in '%s' in '%s': %v", channelName, chainDoc.Name, err)
			}
		}

		// step 5: start listening in the vc
		listenVc(h, i, sess, vcCtx, *voiceState.ChannelID, channelName, chainDoc)
	}()

}

func countVCUsers(h *SlashCommandsHandler, guildID snowflake.ID, channelID snowflake.ID) int {
	n := 0
	for vs := range h.Client.Caches.VoiceStates(guildID) {
		if vs.ChannelID != nil && *vs.ChannelID == channelID && vs.UserID != h.Client.ID() {
			n++
		}
	}
	return n
}

func listenVc(h *SlashCommandsHandler, event *events.ApplicationCommandInteractionCreate, sess *helpers.Session, vcCtx context.Context, channelID snowflake.ID, channelName string, chainConf *guilds.Config) {
	conn := sess.Conn
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
	// Listen for voice state updates to detect when we should leave
	listener := bot.NewListenerFunc(
		func(e events.GuildVoiceStateUpdate) {
			if e.VoiceState.GuildID != guildID {
				return
			}

			if e.VoiceState.UserID == botUserID {
				if ch := e.VoiceState.ChannelID; ch != nil && *ch == channelID {
					// muted, deafened or otherwise edited, still here
					return
				}
				logger.Infof("Left voice channel '%s' in '%s', initiating cleanup...", channelName, chainConf.Name)
				triggerCleanup()
				return
			}

			if countVCUsers(h, guildID, channelID) < 1 { // All other users have left
				triggerCleanup()
			}
		},
	)
	h.Client.EventManager.AddEventListeners(listener)
	if countVCUsers(h, guildID, channelID) < 1 {
		// everyone left while joining, before the listener was there to see it
		triggerCleanup()
	}

	// use the 'done' channel to instruct other goroutines to stop *before* cleanup.
	defer func(updateListener bot.EventListener) {
		h.Client.RemoveEventListeners(updateListener)
		triggerCleanup() // safely closes the done channel exactly once
		sess.Leave()
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

	speak := func() {
		// one utterance at a time; triggers while speaking are dropped
		if !ttsMutex.TryLock() {
			return
		}
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
			logger.Warnf("Generated empty msg in vc handler")
			return
		}
		provider, err := tts.GenerateTTSProvider(msg, chainConf.TTSLanguage)
		if err != nil {
			logger.Errorf("Failed to generate random TTS provider in '%s' in '%s': %v", channelName, chainConf.Name, err)
			return
		}
		if err := sess.Play(vcCtx, provider); err != nil {
			if !errors.Is(err, io.EOF) {
				logger.Errorf("Failed to stream random TTS audio in '%s' in '%s': %v", channelName, chainConf.Name, err)
				triggerCleanup()
			}
		}
	}

	// utterances are stored in order by one worker, off the audio loop: a
	// slow DB write must not stall Opus decoding and drop speech
	utterances := make(chan string, 64)
	go func() {
		for {
			select {
			case text := <-utterances:
				if err := h.ChainsService.UpdateChainState(vcCtx, guildID.String(), []string{text}); err != nil {
					logger.Errorf("Failed to store utterance in '%s': %v", chainConf.Name, err)
				}
				if strings.Contains(text, "rolando") || utils.RatedChoice(chainConf.ReplyRate) {
					go speak()
				}
			case <-done:
				return
			}
		}
	}()
	onUtterance := func(text string) {
		if text == "" {
			return
		}
		select {
		case utterances <- text:
		default:
			logger.Warnf("Utterance queue full in '%s', dropping one", chainConf.Name)
		}
	}

	go func() {
		pcmChan := make(chan *helpers.PCMPacket, 100)
		receiver := helpers.NewSpeechReceiver(pcmChan)
		conn.SetOpusFrameReceiver(receiver)

		speakers := map[snowflake.ID]*speaker{}
		defer func() {
			receiver.Close()
			for _, sp := range speakers {
				sp.rec.Free()
			}
		}()

		ticker := time.NewTicker(speechFlushInterval)
		defer ticker.Stop()
		for {
			select {
			case packet := <-pcmChan:
				if packet.UserID == botUserID {
					continue
				}
				sp, ok := speakers[packet.UserID]
				if !ok {
					rec, err := stt.NewRecognizer(chainConf.TTSLanguage)
					if err != nil {
						logger.Errorf("Failed Speech to Text: %v", err)
						continue
					}
					sp = &speaker{rec: rec}
					speakers[packet.UserID] = sp
				}
				onUtterance(sp.add(packet.PCM))

			case now := <-ticker.C:
				for id, sp := range speakers {
					idle := now.Sub(sp.last)
					if sp.pending && idle >= speechSilence {
						onUtterance(sp.flush())
					}
					if idle >= speakerIdleTTL {
						sp.rec.Free()
						delete(speakers, id)
					}
				}

			case <-done:
				logger.Infof("Received shutdown signal in Audio Processing goroutine, exiting")
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

const (
	speechChunk         = stt.SampleRate * 2 / 5 // 200ms of s16le
	speechFlushInterval = 250 * time.Millisecond
	speechSilence       = 700 * time.Millisecond
	speakerIdleTTL      = 2 * time.Minute
)

type speaker struct {
	rec     *stt.Recognizer
	buf     []byte
	last    time.Time
	pending bool
}

// add buffers a frame and feeds Vosk every speechChunk, returning a finished
// utterance if Vosk detected one.
func (sp *speaker) add(pcm []int16) string {
	for _, v := range pcm {
		sp.buf = append(sp.buf, byte(v), byte(v>>8))
	}
	sp.last = time.Now()
	sp.pending = true
	if len(sp.buf) < speechChunk {
		return ""
	}
	text := sp.rec.Accept(sp.buf)
	sp.buf = sp.buf[:0]
	return text
}

func (sp *speaker) flush() string {
	if len(sp.buf) > 0 {
		sp.rec.Accept(sp.buf)
		sp.buf = sp.buf[:0]
	}
	sp.pending = false
	return sp.rec.Flush()
}
