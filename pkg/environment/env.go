package environment

import "os"

var (
	DiscordAPIKey    = lookupEnv("DISCORD_API_KEY", "fake")
	DiscordGuildID   = lookupEnv("DISCORD_GUILD_ID", "fake")
	DiscordChannelID = lookupEnv("DISCORD_CHANNEL_ID", "fake")
	DiscordMessageID = lookupEnv("DISCORD_MESSAGE_ID", "fake")
)

func lookupEnv(key string, def string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return def
}
