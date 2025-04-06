package commands

import (
	"fmt"
	"rolando/internal/logger"
	"rolando/internal/tts"
	"rolando/internal/utils"
	"sync"
	"time"

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
	chainDoc, _ := h.ChainsService.GetChainDocument(voiceState.GuildID)
	chain, _ := h.ChainsService.GetChain(chainDoc.ID)
	var ttsMutex sync.Mutex
	d, err := tts.GenerateTTSDecoder("i am here", chainDoc.TTSLanguage)
	if err != nil {
		logger.Errorf("Failed to generate TTS decoder: %v", err)
		return
	}
	if err := utils.StreamAudioDecoder(vc, d); err != nil {
		logger.Errorf("Failed to stream audio in '%s' in '%s': %v", voiceChannel.Name, chainDoc.Name, err)
	}

	// step 5: start listening in the vc
	leaveChan := make(chan struct{})
	var cleanupHandler func()
	go func() {
		defer close(leaveChan)
		cleanupHandler = s.AddHandler(func(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
			if vs.GuildID != i.GuildID {
				return // Not the guild we're in
			}
			if vs.UserID == s.State.User.ID {
				return // the bot leaving
			}
			currentUsers, _ := getVCUsers(s, i.GuildID, voiceState.ChannelID)
			if len(currentUsers) < 1 { // All other users have left the vc
				leaveChan <- struct{}{} // Signal to leave the vc
			}
		})
		for packet := range vc.OpusRecv {
			if packet == nil {
				continue
			}
			random := utils.GetRandom(1, 1000)
			if random != 1 {
				continue
			}
			go func() {
				ttsMutex.Lock()
				defer ttsMutex.Unlock()
				d, err := tts.GenerateTTSDecoder(chain.TalkOnlyText(10), chainDoc.TTSLanguage)
				if err != nil {
					logger.Errorf("Failed to generate random TTS decoder in '%s' in '%s': %v", voiceChannel.Name, chainDoc.Name, err)
					return
				}
				if err := utils.StreamAudioDecoder(vc, d); err != nil {
					logger.Errorf("Failed to stream random TTS audio in '%s' in '%s': %v", voiceChannel.Name, chainDoc.Name, err)
				}
			}()
		}
	}()

	// cleanup: leave the vc when receiving the signal or after 8 hours
	select {
	case <-leaveChan:
		logger.Infof("Leaving vc '%s' in '%s'", voiceChannel.Name, chainDoc.Name)
		cleanupHandler()
		vc.Disconnect()
		vc.Close()
		break
	case <-time.After(8 * time.Hour): // timeout after 8 hours
		logger.Infof("VC Timeout in '%s' in '%s'", voiceChannel.Name, chainDoc.Name)
		cleanupHandler()
		vc.Disconnect()
		vc.Close()
		break
	}
}

func getVCUsers(s *discordgo.Session, guildID, channelID string) ([]*discordgo.Member, error) {
	guild, err := s.Guild(guildID)
	if err != nil {
		return nil, err
	}

	var users []*discordgo.Member
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID == channelID {
			for _, member := range guild.Members {
				if member.User.ID == vs.UserID {
					users = append(users, member)
					break
				}
			}
		}
	}
	return users, nil
}
