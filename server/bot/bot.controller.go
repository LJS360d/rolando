package bot

import (
	"fmt"
	"rolando/cmd/log"
	"rolando/cmd/services"
	"rolando/config"
	"rolando/server/auth"
	"runtime"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
)

type BotController struct {
	chainsService *services.ChainsService
	ds            *discordgo.Session
}

func NewController(chainsService *services.ChainsService, ds *discordgo.Session) *BotController {
	return &BotController{
		chainsService: chainsService,
		ds:            ds,
	}
}

type BroadcastRequest struct {
	Content string                   `json:"content"`
	Guilds  []*BroadcastGuildRequest `json:"guilds"`
}

type BroadcastGuildRequest struct {
	Id        string `json:"id"`
	ChannelId string `json:"channel_id"`
}

// POST /bot/broadcast, requires owner authorization
func (s *BotController) Broadcast(c *gin.Context) {
	errCode, err := auth.EnsureOwner(c, s.ds)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}
	req := &BroadcastRequest{}
	err = c.ShouldBindBodyWithJSON(req)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(req.Guilds))

	for _, g := range req.Guilds {
		wg.Add(1)

		go func(g *BroadcastGuildRequest) {
			defer wg.Done()
			channelId := g.ChannelId
			if channelId == "" {
				guild, err := s.ds.Guild(g.Id)
				if err != nil {
					errCh <- err
					return
				}
				channelId = guild.SystemChannelID
			}
			log.Log.Infof("Broadcasting message in guild: %s, channel: %s", g.Id, channelId)
			_, err := s.ds.ChannelMessageSend(channelId, req.Content)
			if err != nil {
				log.Log.Errorf("could not send message in guild: %s, channel: %s: %v", g.Id, channelId, err)
				errCh <- err
			}
		}(g)
	}

	wg.Wait()
	close(errCh)

	// Collect errors, if any
	for err := range errCh {
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(200, gin.H{"content": req.Content})
}

// GET /bot/guilds, requires owner authorization
func (s *BotController) GetBotGuilds(c *gin.Context) {
	errCode, err := auth.EnsureOwner(c, s.ds)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}
	guilds, err := s.ds.UserGuilds(200, "", "", true)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, guilds)
}

// GET /bot/guilds/:guildId/invite, requires owner authorization
func (s *BotController) GetGuildInvite(c *gin.Context) {
	errCode, err := auth.EnsureOwner(c, s.ds)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}

	channels, err := s.ds.GuildChannels(c.Param("guildId"))
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var publicChannelID string
	for _, channel := range channels {
		if channel != nil && channel.Type == discordgo.ChannelTypeGuildText {
			publicChannelID = channel.ID
			break
		}
	}

	if publicChannelID == "" {
		c.JSON(400, gin.H{"error": "No public channels available in the guild"})
		return
	}

	inv, err := s.ds.ChannelInviteCreate(publicChannelID, discordgo.Invite{
		MaxAge:    86400,
		MaxUses:   1,
		Temporary: false,
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"invite": fmt.Sprintf("https://discord.gg/%s", inv.Code)})
}

// GET /bot/user, public
func (s *BotController) GetBotUser(c *gin.Context) {
	botUser := s.ds.State.User
	commands, err := s.ds.ApplicationCommands(s.ds.State.User.ID, "")
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	c.JSON(200, gin.H{
		"id":             botUser.ID,
		"username":       botUser.Username,
		"global_name":    botUser.Username + "#" + botUser.Discriminator,
		"avatar_url":     botUser.AvatarURL(""),
		"discriminator":  botUser.Discriminator,
		"verified":       botUser.Verified,
		"accent_color":   int32(botUser.AccentColor),
		"invite_url":     config.InviteUrl,
		"slash_commands": commands,
		"guilds":         len(s.ds.State.Guilds),
	})
}

// GET /bot/resources, public
func (s *BotController) GetBotResources(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	c.JSON(200, gin.H{
		"startup_timestamp_unix": config.StartupTime.Unix(),
		"cpu_cores":              runtime.NumCPU(),
		"memory": gin.H{
			"total_alloc":  m.TotalAlloc,
			"sys":          m.Sys,
			"heap_alloc":   m.HeapAlloc,
			"heap_sys":     m.HeapSys,
			"stack_in_use": m.StackInuse,
			"gc_count":     m.NumGC,
		},
	})
}

// DELETE /bot/guild/:guildId, requires owner authorization
func (s *BotController) LeaveGuild(c *gin.Context) {
	errCode, err := auth.EnsureOwner(c, s.ds)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}
	guildId := c.Param("guildId")
	err = s.ds.GuildLeave(guildId)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(204, nil)
	err = s.chainsService.DeleteChain(guildId)
	if err != nil {
		log.Log.Errorf("Failed to delete chain after leaving guild: %v", err)
		return
	}
}
