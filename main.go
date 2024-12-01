package main

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sokdak/eternity-bot/pkg/environment"
	"github.com/sokdak/eternity-bot/pkg/handler"
)

func main() {
	dg, err := discordgo.New("Bot " + environment.DiscordAPIKey)
	if err != nil {
		panic(err)
	}

	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers

	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening connection,", err)
		return
	}
	defer dg.Close()

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			//err := handler.UpdateMessageWithRoles(dg, environment.DiscordGuildID,
			//	environment.DiscordChannelID, environment.DiscordMessageID)
			//if err != nil {
			//	fmt.Println("Error updating message:", err)
			//}
			//err = handler.GeneralizeUsername(dg, environment.DiscordGuildID)
			//if err != nil {
			//	fmt.Println("Error generalizing username:", err)
			//}
		}
	}
	return
}
