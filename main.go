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

var messagePull *MessagePull

func init() {
	var err error
	log.Println("Loading app config")
	appConfig, err = readAppConfig()
	if err != nil {
		log.Printf("Error occured at init : %v", err)
		log.Fatal("Cant run app due to fatal errors during init")
	}
	messagePull = NewMessagePull(cleanQuestionPoolInterval, inlineTempQuestionStoreTime)
	messagePull.init()
}

func main() {
	sqlstore, err := NewSQLStore("botbase.sql")
	if err != nil {
		log.Fatal(err)
	}
	defer sqlstore.db.Close()
	bot, err := tgbotapi.NewBotAPI(appConfig.TelegramBotToken)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		log.Println("Receive update")
		go processUpdate(bot, &update, sqlstore)
	}

}

func processUpdate(bot *tgbotapi.BotAPI, update *tgbotapi.Update, store *SQLStore) (err error) {
	if update.CallbackQuery != nil {
		log.Println("Receive callback")
		var reply string
		reply = proccessCallback(bot, update.CallbackQuery, store)

		err = sendCallbackNotification(bot, update.CallbackQuery.ID, reply)
		if err != nil {
			log.Printf("Error sending notification")
			return
		}
		return
	}
	if (update.Message == nil) && (update.InlineQuery == nil) {
		return
	}
	if update.InlineQuery != nil {

		log.Println(update.InlineQuery.ID)
		err = processInlineQuery(bot, update, store)
		if err != nil {
			log.Println(err)
		}
	} else {

		if isUserChat(update.Message.Chat) {
			log.Println("Received message from user chat")
			_, err = store.getUserChatID(update.Message.From.UserName)
			log.Println("Find it's personal chatID in database")
			if err != nil {
				log.Println(err)
				log.Println("Adding user chatID with fbbbot to database")
				user := &User{
					ID:     update.Message.From.ID,
					Name:   strings.Replace(update.Message.From.UserName, "@", "", -1),
					ChatID: update.Message.Chat.ID,
				}
				err = store.addUser(user)
				if err != nil {
					log.Printf("Error accesing database: %v", err)
					return
				}
			}
		} else {
			log.Println(update.Message.Chat.ID)
		}

		if !update.Message.IsCommand() {
			if strings.HasPrefix(update.Message.Text, "------") {
				log.Println("Recognised as inline answer")
				deleteConfig := tgbotapi.DeleteMessageConfig{
					ChatID:    update.Message.Chat.ID,
					MessageID: update.Message.MessageID,
				}
				go messageDeleter(bot, deleteConfig, appConfig.InlineAnswersTimeToDelete)
			}
		} else {
			log.Println("Recognised as command")
			err = processCommand(bot, update, store)
			if err != nil {
				log.Println(err)
			}
		}
	}
	return
}

