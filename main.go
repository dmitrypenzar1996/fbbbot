package main

import (
	"encoding/json"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"io/ioutil"
	"log"
	"strings"
)

func readAppConfig() (config *AppConfig, err error) {
	config_json, err := ioutil.ReadFile(appConfigPath)
	if err != nil {
		return
	}

	config = new(AppConfig)
	err = json.Unmarshal(config_json, config)
	if err != nil {
		return
	}
	return
}

func init() {
	var err error
	log.Println("Loading app config")
	appConfig, err = readAppConfig()
	if err != nil {
		log.Printf("Error occured at init : %v", err)
		log.Fatal("Cant run app due to fatal errors during init")
	}
}

func main() {
	sqlstore, err := NewSQLStore("botbase.sql")
	if err != nil {
		log.Fatal(err)
	}
	defer sqlstore.db.Close()
	bot, err := tgbotapi.NewBotAPI(appConfig.TelegramBotToken)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		log.Println("receive update")
		go processUpdate(bot, &update, sqlstore)
	}

}

func processUpdate(bot *tgbotapi.BotAPI, update *tgbotapi.Update, store *SQLStore) (err error) {

	log.Println("Receive update")

	if update.CallbackQuery != nil {
		log.Println("Receive callback")

		return
	}
	if (update.Message == nil) && (update.InlineQuery == nil) {
		return
	}

	if update.InlineQuery != nil {
		log.Println("Recognised as inline query")
		err = processInlineQuery(bot, update, store)
		if err != nil {
			log.Println(err)
		}
	} else if !update.Message.IsCommand() {
		if strings.HasPrefix(update.Message.Text, "------") {
			deleteConfig := tgbotapi.DeleteMessageConfig{
				ChatID:    update.Message.Chat.ID,
				MessageID: update.Message.MessageID,
			}
			go messageDeleter(bot, deleteConfig, appConfig.AnswersTimeToDelete)
		}
	} else {
		err = processCommand(bot, update, store)
		if err != nil {
			log.Println(err)
		}
	}
	return
}
