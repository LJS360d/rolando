package bot

import (
	"fmt"
	"rolando/cmd/idiscord/services"
	"rolando/cmd/ihttp/analytics"
	"rolando/cmd/ihttp/auth"
	"rolando/internal/config"
	"rolando/internal/logger"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"sync"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/gin-gonic/gin"
)

type BotController struct {
	chainsService *services.ChainsService
	ds            *bot.Client
}

func NewController(chainsService *services.ChainsService, ds *bot.Client) *BotController {
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
			channelId, err := snowflake.Parse(g.ChannelId)
			if err != nil {
				errCh <- err
				return
			}
			gid, err := snowflake.Parse(g.Id)
			if err != nil {
				errCh <- err
				return
			}
			guild, ok := s.ds.Caches.Guild(gid)
			if !ok {
				errCh <- fmt.Errorf("guild with id '%s' not found in cache", g.Id)
				return
			}
			if guild.SystemChannelID == nil {
				errCh <- fmt.Errorf("guild with id '%s' has no system channel", g.Id)
				return
			}
			channelId = *guild.SystemChannelID

			logger.Infof("Broadcasting message in guild: %s, channel: %s", g.Id, channelId)
			_, err = s.ds.Rest.CreateMessage(channelId, discord.NewMessageCreate().WithContent(req.Content))
			if err != nil {
				logger.Errorf("could not send message in guild: %s, channel: %s: %v", g.Id, channelId, err)
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

const discordUserGuildsLimit = 200

func sortKeyGuildWithChain(g discord.Guild, chain gin.H, sortBy string) float64 {
	if sortBy == "approximate_member_count" {
		return float64(g.ApproximateMemberCount)
	}
	if sortBy == "approximate_presence_count" {
		return float64(g.ApproximatePresenceCount)
	}
	if chain == nil {
		return -1
	}
	v, ok := chain[sortBy]
	if !ok {
		return -1
	}
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case uint32:
		return float64(x)
	case uint64:
		return float64(x)
	case float64:
		return x
	default:
		return -1
	}
}

// GET /bot/guilds, requires owner authorization
func (s *BotController) GetBotGuildsPaginated(c *gin.Context) {
	errCode, err := auth.EnsureOwner(c, s.ds)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}
	pageSize, err := strconv.Atoi(c.Query("pageSize"))
	if err != nil || pageSize <= 0 {
		pageSize = 10
	}
	page, err := strconv.Atoi(c.Query("page"))
	if err != nil || page < 1 {
		page = 1
	}
	sortBy := c.Query("sortBy")

	chainMap, err := analytics.BuildAllChainsAnalyticsMap(s.chainsService)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	type guildWithChain struct {
		Guild   discord.Guild
		Chain   gin.H
		SortKey float64
	}
	allGuilds := slices.Collect(s.ds.Caches.Guilds())
	merged := make([]guildWithChain, 0, len(allGuilds))
	for _, g := range allGuilds {
		chain := chainMap[g.ID.String()]
		merged = append(merged, guildWithChain{
			Guild:   g,
			Chain:   chain,
			SortKey: sortKeyGuildWithChain(g, chain, sortBy),
		})
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].SortKey > merged[j].SortKey })

	total := int64(len(merged))
	offset := min((page-1)*pageSize, len(merged))
	end := min(offset+pageSize, len(merged))
	window := merged[offset:end]
	pageData := make([]gin.H, 0, len(window))
	for _, m := range window {
		item := gin.H{
			"id":                         m.Guild.ID,
			"name":                       m.Guild.Name,
			"icon":                       m.Guild.Icon,
			"owner":                      m.Guild.OwnerID.String(),
			"permissions":                "",
			"features":                   m.Guild.Features,
			"approximate_member_count":   m.Guild.MemberCount,
			"approximate_presence_count": m.Guild.ApproximatePresenceCount,
		}
		if m.Chain != nil {
			item["chain"] = m.Chain
		} else {
			item["chain"] = nil
		}
		pageData = append(pageData, item)
	}

	c.JSON(200, gin.H{
		"data": pageData,
		"meta": gin.H{
			"page":       page,
			"pageSize":   pageSize,
			"totalItems": total,
			"totalPages": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// GET /bot/guilds/all, requires owner authorization
func (s *BotController) GetBotGuildsAll(c *gin.Context) {
	errCode, err := auth.EnsureOwner(c, s.ds)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, slices.Collect(s.ds.Caches.Guilds()))
}

// GET /bot/guilds/:guildId, requires member authorization
func (s *BotController) GetGuild(c *gin.Context) {
	guildId := c.Param("guildId")
	errCode, err := auth.EnsureGuildMember(c, s.ds, guildId)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}
	gid, err := snowflake.Parse(guildId)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	guild, ok := s.ds.Caches.Guild(gid)
	if !ok {
		c.JSON(400, gin.H{"error": fmt.Errorf("guild with id '%s' not found in cache", guildId)})
		return
	}
	c.JSON(200, guild)
}

// PUT /bot/guilds/:guildId, requires owner authorization
func (s *BotController) UpdateChainDoc(c *gin.Context) {
	errCode, err := auth.EnsureOwner(c, s.ds)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}
	guildId := c.Param("guildId")

	var fields map[string]any
	err = c.ShouldBindJSON(&fields)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// DB fields update
	chainDoc, err := s.chainsService.UpdateChainMeta(guildId, fields)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, chainDoc)
}

// GET /bot/guilds/:guildId/invite, requires owner authorization
func (s *BotController) GetGuildInvite(c *gin.Context) {
	errCode, err := auth.EnsureOwner(c, s.ds)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}

	gid, err := snowflake.Parse(c.Param("guildId"))
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	channels := slices.Collect(s.ds.Caches.ChannelsForGuild(gid))
	var publicChannelID *snowflake.ID
	for _, channel := range channels {
		if channel != nil && channel.Type() == discord.ChannelTypeGuildText {
			channelId := channel.ID()
			publicChannelID = &channelId
			break
		}
	}

	if publicChannelID == nil {
		c.JSON(400, gin.H{"error": "No public channels available in the guild"})
		return
	}

	inv, err := s.ds.Rest.CreateInvite(*publicChannelID, discord.InviteCreate{
		MaxAge:    new(86400),
		MaxUses:   new(1),
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
	botUser, _ := s.ds.Caches.SelfUser()
	commands, err := s.ds.Rest.GetGlobalCommands(s.ds.ApplicationID, false)
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
		"avatar_url":     botUser.AvatarURL(),
		"discriminator":  botUser.Discriminator,
		"verified":       botUser.Verified,
		"accent_color":   botUser.AccentColor,
		"invite_url":     config.InviteUrl,
		"slash_commands": commands,
		"guilds":         s.ds.Caches.GuildCache().Len(),
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
	guildId, err := snowflake.Parse(c.Param("guildId"))
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	err = s.ds.Rest.LeaveGuild(guildId)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(204, nil)
	err = s.chainsService.DeleteChain(guildId.String())
	if err != nil {
		logger.Errorf("Failed to delete chain after leaving guild: %v", err)
		return
	}
}
