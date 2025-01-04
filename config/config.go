package config

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var (
	Token         string
	Intents       discordgo.Intent
	OwnerIDs      []string
	Version       string
	InviteUrl     string
	Env           string
	DatabasePath  string
	ServerAddress string
	StartupTime   time.Time
)

func init() {
	log.Println("Initializing configuration...")
	// Load environment variables from the .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Assign the environment variables to package-level variables
	Token = os.Getenv("TOKEN")
	if Token == "" {
		log.Fatalf("TOKEN not set in the environment")
	}
	InviteUrl = os.Getenv("INVITE_URL")
	if InviteUrl == "" {
		log.Println("INVITE_URL not set in the environment")
	}
	ownerIDsStr := os.Getenv("OWNER_IDS")
	if ownerIDsStr == "" {
		log.Println("OWNER_IDS not set in the environment")
	} else {
		OwnerIDs = strings.Split(ownerIDsStr, ",")
	}
	DatabasePath = os.Getenv("DATABASE_PATH")
	if DatabasePath == "" {
		log.Println("DATABASE_PATH not set in the environment")
		DatabasePath = "rolando.db"
	}
	ServerAddress = os.Getenv("SERVER_ADDRESS")
	if ServerAddress == "" {
		log.Println("SERVER_ADDRESS not set in the environment")
		ServerAddress = "127.0.0.1:8080"
	}
	StartupTime = time.Now()

	Intents = (discordgo.IntentDirectMessageReactions |
		discordgo.IntentDirectMessageTyping |
		discordgo.IntentDirectMessages |
		// discordgo.IntentAutoModerationConfiguration |
		// discordgo.IntentAutoModerationExecution |
		// discordgo.IntentDirectMessageReactions |
		// discordgo.IntentGuildEmojisAndStickers |
		// discordgo.IntentGuildIntegrations |
		discordgo.IntentGuildInvites |
		// discordgo.IntentGuildMembers |
		discordgo.IntentGuildMessageReactions |
		discordgo.IntentGuildMessageTyping |
		discordgo.IntentGuildMessages |
		// discordgo.IntentGuildModeration |
		// discordgo.IntentGuildPresences |
		// discordgo.IntentGuildScheduledEvents |
		// discordgo.IntentGuildVoiceStates |
		// discordgo.IntentGuildWebhooks |
		discordgo.IntentGuilds |
		discordgo.IntentMessageContent)

}
