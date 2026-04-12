package config

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
	"github.com/joho/godotenv"
)

var (
	Token                string
	Intents              gateway.Intents
	OwnerIDs             []string
	Version              string
	InviteUrl            string
	Env                  string
	DatabasePath         string
	RedisUrl             string
	ServerAddress        string
	LogWebhook           string
	StartupTime          time.Time
	RunHttpServer        bool
	PaywallsEnabled      bool
	VoiceChatFeaturesSKU snowflake.ID
	PremiumsPageLink     string
)

func init() {
	log.Println("Initializing config...")
	// Load environment variables from the .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	Env = os.Getenv("GO_ENV")
	if Env == "" {
		log.Println("GO_ENV not set in the environment")
		Env = "production"
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
	LogWebhook = os.Getenv("LOG_WEBHOOK")
	if LogWebhook == "" {
		log.Println("LOG_WEBHOOK not set in the environment")
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
	RedisUrl = os.Getenv("REDIS_URL")
	if RedisUrl == "" {
		log.Println("REDIS_URL not set in the environment")
		RedisUrl = "redis://localhost:6379"
	}
	ServerAddress = os.Getenv("SERVER_ADDRESS")
	if ServerAddress == "" {
		ServerAddress = "127.0.0.1:8080"
	}
	RunHttpServer = os.Getenv("RUN_HTTP_SERVER") == "true" || os.Getenv("RUN_HTTP_SERVER") == "1" || os.Getenv("RUN_HTTP_SERVER") == ""
	StartupTime = time.Now()

	PaywallsEnabled = os.Getenv("PAYWALLS_ENABLED") == "true" || os.Getenv("PAYWALLS_ENABLED") == "1" || os.Getenv("PAYWALLS_ENABLED") == ""
	voiceChatFeaturesSKUStr := os.Getenv("VOICE_CHAT_FEATURES_SKU_ID")
	if voiceChatFeaturesSKUStr != "" {
		VoiceChatFeaturesSKU, err = snowflake.Parse(voiceChatFeaturesSKUStr)
		if err != nil {
			log.Printf("VOICE_CHAT_FEATURES_SKU_ID '%s' is invalid snowflake\n", voiceChatFeaturesSKUStr)
		}
	}

	PremiumsPageLink = os.Getenv("PREMIUMS_PAGE_LINK")
	Intents = (gateway.IntentDirectMessageReactions |
		gateway.IntentDirectMessageTyping |
		gateway.IntentDirectMessages |
		gateway.IntentGuildVoiceStates |
		// gateway.IntentAutoModerationConfiguration |
		// gateway.IntentAutoModerationExecution |
		// gateway.IntentDirectMessageReactions |
		// gateway.IntentGuildEmojisAndStickers |
		gateway.IntentGuildIntegrations |
		gateway.IntentGuildInvites |
		// gateway.IntentsGuildMembers |
		gateway.IntentGuildMessageReactions |
		gateway.IntentGuildMessageTyping |
		gateway.IntentGuildMessages |
		// gateway.IntentGuildModeration |
		// gateway.IntentGuildPresences |
		// gateway.IntentGuildScheduledEvents |
		// gateway.IntentGuildVoiceStates |
		gateway.IntentGuildWebhooks |
		gateway.IntentGuilds |
		gateway.IntentMessageContent)

	log.Println("Config initialized")
}
