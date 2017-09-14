package main

import (
	"database/sql"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strconv"
	"strings"
)

func makeCallbackData(command string, mHash string) string {
	return command + CallbackDataDelimiter + mHash
}

func parseCallbackData(data string) (command string, mHash string, err error) {
	parts := strings.SplitN(data, CallbackDataDelimiter, 2)
	if len(parts) != 2 {
		err = WrongCallbackDataFormat
		return
	}
	command = parts[0]
	mHash = parts[1]
	return
}

func processQuestionCallback(bot *tgbotapi.BotAPI, question *Question, store *SQLStore) (reply string, err error) {
	question.QuestionID, err = store.addQuestion(question)
	if err != nil {
		log.Printf("Error adding question database: %v", err)
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = "Вопрос успешно добавлен"

	var chatID int64
	if question.Rec.User == AllGroupName {
		chatID = AllGroupChatID
	} else {
		chatID, err = store.getUserChatID(question.Rec.User)
		if err == sql.ErrNoRows {
			log.Printf("Don't know user personal chat adress: %v", err)
			return
		} else if err != nil {
			log.Printf("Error accesing sql database : %v", err)
			return
		}
	}

	msg := makeAskedPersonNotification(question, chatID)
	_, err = bot.Send(msg)

	if err != nil {
		log.Printf("Error sending notification")
		return
	}

	return
}

func processAnswerCallback(bot *tgbotapi.BotAPI, answer *Answer, store *SQLStore) (reply string, err error) {

	question, err := store.getQuestion(answer.QuestionID)
	if err == QuestionDoesntExist {
		reply = fmt.Sprintf("Вопроса с id %d не существует", answer.QuestionID)
	}


	answer.AnswerID, err = store.addAnswer(answer)
	if err != nil {
		reply = "Ошибка доступа к базе данных"
		return
	}

	var chatID int64

	chatID, err = store.getUserChatID(question.User)
	if err == sql.ErrNoRows {
		log.Printf("Don't know user personal chat adress: %v", err)
		return
	} else if err != nil {
		log.Printf("Error accesing sql database : %v", err)
		return
	}

	msg := makeAskerNotification(answer, question, chatID)
	_, err = bot.Send(msg)

	if err != nil {
		log.Printf("Error sending notification")
		return
	}

	msg = makeAskerNotification(answer, question, AllGroupChatID)
	_, err = bot.Send(msg)

	if err != nil {
		log.Printf("Error sending notification")
		return
	}

	return

}

func proccessCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, store *SQLStore) (reply string) {
	log.Println(query.InlineMessageID)
	log.Println(query.Message)

	command, mHash, err := parseCallbackData(query.Data)
	if err != nil {
		log.Printf("Error processing callback data: %v", err)
		reply = "Ошибка приложения"
		return
	}

	switch command {
	case CallbackCancelCommand:
			messagePull.Delete(mHash)
			reply = "Команда успешно отменена"
			return
	case CallbackAddCommand:
			reply, err = processCallbackAddComand(bot, store, mHash)
			return
	case CallbackCloseCommand:
			reply, err = processCallbackCloseCommand(bot, store, mHash, query.From.UserName)
			return
	default:
		reply = "Ошибка приложения"
		err = WrongValue
	}
	return
}

func processCallbackCloseCommand(bot *tgbotapi.BotAPI, store *SQLStore, mHash string,
	user string) (reply string, err error) {
	qID, err := strconv.Atoi(mHash)
	if err != nil {
		log.Printf("Error while converting qID: %v", err)
		reply = "Ошибка приложения"
		return
	}

	question, err := store.getQuestion(qID)
	if err == QuestionDoesntExist {
		log.Printf("Error getting question from database: %v", err)
		reply = "Ошибка приложения"
		return
	}

	if question.User != user && question.Rec.User != user && !inGroup(appConfig.Admins, user) {
		reply = "Недостаточно прав"
		return
	}

	err = store.closeQuestion(qID)
	if err != nil {
		log.Printf("Error deleting question from database: %v", err)
		reply = "Ошибка приложения"
		return
	}
	reply = "Вопрос успешно закрыт"

	notificationText := fmt.Sprintf("Ваш вопрос [%d] был закрыт: \n%s",
		qID, question.Text)

	log.Println(question.ChatID)
	err = sendSimpleNotification(bot, notificationText, question.ChatID)
	if err != nil {
		log.Printf("Error sending notification")
	}
	return
}

func sendSimpleNotification(bot *tgbotapi.BotAPI, messageText string, chatID int64) (err error) {
	msg := tgbotapi.NewMessage(chatID, messageText)
	resMsg, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending notification: %v", err)
		return
	}
	config := tgbotapi.DeleteMessageConfig{
		MessageID: resMsg.MessageID,
		ChatID:    resMsg.Chat.ID,
	}
	go messageDeleter(bot, config, appConfig.NotificationsTimeToDelete)
	return
}

func processCallbackAddComand(bot *tgbotapi.BotAPI, store *SQLStore, messageHash string) (reply string, err error) {
	m, err := messagePull.getMessage(messageHash)
	if err != nil {
		log.Printf("Access to deleted question: %v", err)
		reply = "К сожалению, ваш вопрос был удален из временной базы"
		return
	}

	switch m.(type) {
	case *Question:
		log.Println("Adding question")
		question := m.(*Question)
		reply, err = processQuestionCallback(bot, question, store)
	case *Answer:
		log.Println("Adding answer")
		answer := m.(*Answer)
		reply, err = processAnswerCallback(bot, answer, store)
	}
	return
}

func sendCallbackNotification(bot *tgbotapi.BotAPI, callbackID string, message_text string) (err error) {
	config := tgbotapi.CallbackConfig{
		CallbackQueryID: callbackID,
		Text:            message_text,
	}

	_, err = bot.AnswerCallbackQuery(config)
	if err != nil {
		return
	}
	return
}
