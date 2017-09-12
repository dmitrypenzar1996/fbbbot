package main

import (
	"log"
	"time"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

func inGroup(userList []string, user string) (flag bool) {
	for _, value := range userList {
		if value == user {
			flag = true
			return
		}
	}
	flag = false
	return
}

func messageDeleter(bot *tgbotapi.BotAPI, config tgbotapi.DeleteMessageConfig, waitTime int) {
	time.Sleep(time.Second * time.Duration(waitTime))
	log.Printf("Deleting message %d from chat %d", config.MessageID, config.ChatID)
	_, err := bot.DeleteMessage(config)
	if err != nil {
		log.Println(err)
		return
	}
}


func formatDate(d time.Time) (s string){
	s = d.Local().Format(dateFormat)
	return
}