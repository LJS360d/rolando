package server

import (
	"rolando/cmd/log"
	"rolando/cmd/repositories"
	"rolando/cmd/services"
	"rolando/config"
	"rolando/server/analytics"
	"rolando/server/auth"
	"rolando/server/bot"
	"rolando/server/data"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
)

type HttpServer struct {
	ChainsService  *services.ChainsService
	DiscordSession *discordgo.Session
	MessagesRepo   *repositories.MessagesRepository
	engine         *gin.Engine
}

func NewHttpServer(discordSession *discordgo.Session, chainsService *services.ChainsService, messagesRepo *repositories.MessagesRepository) *HttpServer {
	return &HttpServer{
		ChainsService:  chainsService,
		DiscordSession: discordSession,
		MessagesRepo:   messagesRepo,
	}
}

func (s *HttpServer) Start() {
	if config.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	s.engine = r

	analyticsController := analytics.NewController(s.ChainsService, s.DiscordSession)
	botController := bot.NewController(s.ChainsService, s.DiscordSession)
	authController := auth.NewController(s.DiscordSession)
	dataController := data.NewController(s.DiscordSession, s.MessagesRepo)
	// Routes
	r.GET("/auth/@me", authController.GetUser)

	r.GET("/analytics/:chain", analyticsController.GetChainAnalytics)
	r.GET("/analytics", analyticsController.GetAllChainsAnalytics)

	r.GET("/data/:chain/all", dataController.GetData)
	r.GET("/data/:chain", dataController.GetDataPaginated)

	r.GET("/bot/user", botController.GetBotUser)
	r.GET("/bot/guilds", botController.GetBotGuilds)
	r.GET("/bot/guilds/:guildId", botController.GetGuild)
	r.PUT("/bot/guilds/:guildId", botController.UpdateChainDoc)
	r.DELETE("/bot/guilds/:guildId", botController.LeaveGuild)
	r.GET("/bot/guilds/:guildId/invite", botController.GetGuildInvite)

	r.GET("/bot/resources", botController.GetBotResources)
	r.POST("/bot/broadcast", botController.Broadcast)

	// Start the server
	log.Log.Infof("Server listening at %v", config.ServerAddress)
	if err := r.Run(config.ServerAddress); err != nil {
		log.Log.Fatalf("failed to start server: %v", err)
	}
}
