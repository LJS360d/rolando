package bot

import (
	"fmt"
	"rolando/cmd/idiscord/services"
	"rolando/cmd/ihttp/analytics"
	"rolando/cmd/ihttp/auth"
	"rolando/internal/config"
	"rolando/internal/logger"
	"runtime"
	"sort"
	"strconv"
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
			logger.Infof("Broadcasting message in guild: %s, channel: %s", g.Id, channelId)
			_, err := s.ds.ChannelMessageSend(channelId, req.Content)
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

func fetchAllUserGuilds(ds *discordgo.Session) ([]*discordgo.UserGuild, error) {
	var all []*discordgo.UserGuild
	afterID := ""
	for {
		page, err := ds.UserGuilds(discordUserGuildsLimit, "", afterID, true)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < discordUserGuildsLimit {
			break
		}
		afterID = page[len(page)-1].ID
	}
	return all, nil
}

func sortKeyGuildWithChain(g *discordgo.UserGuild, chain gin.H, sortBy string) float64 {
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

// GET /bot/guilds, requires owner authorization. Query: page, pageSize, sortBy (chain or approximate_member_count).
func (s *BotController) GetBotGuildsPaginated(c *gin.Context) {
	errCode, err := auth.EnsureOwner(c, s.ds)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}
	pageSize, err := strconv.Atoi(c.Query("pageSize"))
	if err != nil || pageSize <= 0 {
		pageSize = 24
	}
	page, err := strconv.Atoi(c.Query("page"))
	if err != nil || page < 1 {
		page = 1
	}
	sortBy := c.Query("sortBy")
	if sortBy == "" {
		sortBy = "bytes"
	}

	allGuilds, err := fetchAllUserGuilds(s.ds)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	chainMap, err := analytics.BuildAllChainsAnalyticsMap(s.chainsService)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	type guildWithChain struct {
		Guild   *discordgo.UserGuild
		Chain   gin.H
		SortKey float64
	}
	merged := make([]guildWithChain, 0, len(allGuilds))
	for _, g := range allGuilds {
		chain := chainMap[g.ID]
		merged = append(merged, guildWithChain{
			Guild:   g,
			Chain:   chain,
			SortKey: sortKeyGuildWithChain(g, chain, sortBy),
		})
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].SortKey > merged[j].SortKey })

	total := int64(len(merged))
	offset := (page - 1) * pageSize
	if offset > len(merged) {
		offset = len(merged)
	}
	end := offset + pageSize
	if end > len(merged) {
		end = len(merged)
	}
	window := merged[offset:end]
	pageData := make([]gin.H, 0, len(window))
	for _, m := range window {
		item := gin.H{
			"id":                         m.Guild.ID,
			"name":                       m.Guild.Name,
			"icon":                       m.Guild.Icon,
			"owner":                      m.Guild.Owner,
			"permissions":                m.Guild.Permissions,
			"features":                   m.Guild.Features,
			"approximate_member_count":   m.Guild.ApproximateMemberCount,
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
	all, err := fetchAllUserGuilds(s.ds)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, all)
}

// GET /bot/guilds/:guildId, requires member authorization
func (s *BotController) GetGuild(c *gin.Context) {
	guildId := c.Param("guildId")
	errCode, err := auth.EnsureGuildMember(c, s.ds, guildId)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}

	guild, err := s.ds.State.Guild(guildId)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	userGuild := discordgo.UserGuild{
		ID:                       guild.ID,
		Name:                     guild.Name,
		Icon:                     guild.Icon,
		Features:                 guild.Features,
		Owner:                    guild.Owner,
		Permissions:              guild.Permissions,
		ApproximateMemberCount:   guild.ApproximateMemberCount,
		ApproximatePresenceCount: guild.ApproximatePresenceCount,
	}

	c.JSON(200, userGuild)
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
		logger.Errorf("Failed to delete chain after leaving guild: %v", err)
		return
	}
}
