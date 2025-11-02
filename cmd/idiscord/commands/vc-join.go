package commands

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"rolando/cmd/idiscord/helpers"
	"rolando/internal/logger"
	"rolando/internal/model"
	"rolando/internal/repositories"
	"rolando/internal/stt"
	"rolando/internal/tts"
	"rolando/internal/utils"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/dgvoice"
	"github.com/bwmarrin/discordgo"
)

// implementation of /vc join command
func (h *SlashCommandsHandler) vcJoinCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// step 0: defer a response to the interaction
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	// step 1: get the user's voice state
	voiceState, err := s.State.VoiceState(i.GuildID, i.Member.User.ID)
	if err != nil {
		content := "You must be in a voice channel to use this command."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return
	}

	// step 2: join the voice channel
	vc, err := s.ChannelVoiceJoin(i.GuildID, voiceState.ChannelID, false, false)
	if err != nil || !vc.Ready {
		channel, _ := s.State.Channel(voiceState.ChannelID)
		content := fmt.Sprintf("Failed to join the voice channel: %s", channel.Name)
		logger.Errorln(content, err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return
	}
	voiceChannel, _ := s.State.Channel(voiceState.ChannelID)
	// step 3: having joined the vc, respond to the interaction
	content := fmt.Sprintf("Joined the voice channel '%s', i'll speak sometimes", voiceChannel.Name)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})

	// step 4: generate a static TTS audio and stream it to the vc
	chainDoc, err := h.ChainsService.GetChainDocument(voiceState.GuildID)
	if err != nil {
		logger.Errorf("Failed to retrieve chain document: %v", err)
		return
	}
	chain, err := h.ChainsService.GetChain(chainDoc.ID)
	if err != nil {
		logger.Errorf("Failed to retrieve chain: %v", err)
		return
	}
	d, err := tts.GenerateTTSDecoder("i am here", chainDoc.TTSLanguage)
	if err != nil {
		logger.Errorf("Failed to generate TTS decoder: %v", err)
		return
	}
	if err := helpers.StreamAudioDecoder(vc, d); err != nil {
		logger.Errorf("Failed to stream audio in '%s' in '%s': %v", voiceChannel.Name, chainDoc.Name, err)
	}

	// step 5: start listening in the vc
	listenVc(s, i, vc, voiceChannel, chainDoc, chain)
}

func getVCUsers(s *discordgo.Session, guildID, channelID string) ([]*discordgo.Member, error) {
	guild, err := s.Guild(guildID)
	if err != nil {
		return nil, err
	}

	memberMap := make(map[string]*discordgo.Member)
	for _, member := range guild.Members {
		memberMap[member.User.ID] = member
	}

	var users []*discordgo.Member
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID == channelID {
			if member, ok := memberMap[vs.UserID]; ok {
				users = append(users, member)
			}
		}
	}
	return users, nil
}

func listenVc(s *discordgo.Session, i *discordgo.InteractionCreate, vc *discordgo.VoiceConnection, voiceChannel *discordgo.Channel, chainDoc *repositories.Chain, chain *model.MarkovChain) {
	var ttsMutex sync.Mutex
	done := make(chan struct{})

	freeCleanupHandler := s.AddHandler(func(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
		if vs.GuildID != i.GuildID {
			// not the guild we are working in, ignore
			return
		}

		if vs.UserID == s.State.User.ID {
			// the bot leaving through other means (kicked, /vc-leave, lost connection, etc.) shutdown
			logger.Infof("Left voice channel '%s' in '%s', initiating cleanup...", voiceChannel.Name, chainDoc.Name)
			select {
			case done <- struct{}{}: // Send signal if possible
			default: // Don't block if already sent/closed
			}
			return
		}

		currentUsers, _ := getVCUsers(s, i.GuildID, vc.ChannelID)
		if len(currentUsers) < 1 { // All other users have left
			select {
			case done <- struct{}{}: // Send signal if possible
			default: // Don't block if already sent/closed
			}
		}
	})

	// use the 'done' channel to instruct other goroutines to stop *before* cleanup.
	defer func() {
		freeCleanupHandler()
		select {
		case done <- struct{}{}: // Send signal if possible
		default: // Don't block if already sent/closed
		}
		close(done)
		err := vc.Disconnect()
		if err != nil {
			logger.Warnf("Failed to disconnect from voice channel (already disconnected?): %v", err)
		}
		stt.FreeRecognizer(chain.ID)
		logger.Infof("Cleanup complete for VC '%s' in '%s'", voiceChannel.Name, chainDoc.Name)
	}()

	go func() {
		pcmChan := make(chan *discordgo.Packet)
		go dgvoice.ReceivePCM(vc, pcmChan)

		for {
			select {
			case packet, ok := <-pcmChan:
				if !ok || packet == nil {
					// Connection dropped or dgvoice finished. Signal shutdown.
					logger.Warnf("PCM channel closed. initiating cleanup...")
					select {
					case done <- struct{}{}:
					default:
					}
					return
				}

				pcm := packet.PCM
				var audioData bytes.Buffer
				binary.Write(&audioData, binary.LittleEndian, pcm)
				text, err := stt.SpeechToTextNative(&audioData, chainDoc.TTSLanguage, chainDoc.ID)

				if err != nil {
					logger.Errorf("Failed Speech to Text: %v", err)
					continue
				}

				random := utils.GetRandom(1, 1000)
				if text != "" {
					chain.UpdateState(text)
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
					d, err := tts.GenerateTTSDecoder(chain.TalkFiltered(10), chainDoc.TTSLanguage)
					if err != nil {
						logger.Errorf("Failed to generate random TTS decoder in '%s' in '%s': %v", voiceChannel.Name, chainDoc.Name, err)
						return
					}
					if err := helpers.StreamAudioDecoder(vc, d); err != nil {
						logger.Errorf("Failed to stream random TTS audio in '%s' in '%s': %v", voiceChannel.Name, chainDoc.Name, err)
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
		logger.Infof("VC Timeout in '%s' in '%s', initiating cleanup...", voiceChannel.Name, chainDoc.Name)
	}
}
