package main

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strings"
)

func processCommand(bot *tgbotapi.BotAPI, update *tgbotapi.Update, store *SQLStore) (err error) {

	log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

	reply, err := commandExec(bot, update, store)

	log.Println(err)
	if err != nil {
		deleteConfig := tgbotapi.DeleteMessageConfig{
			ChatID:    update.Message.Chat.ID,
			MessageID: update.Message.MessageID,
		}

		go messageDeleter(bot, deleteConfig, appConfig.ErrorsTimeToDelete)
	}
	if reply != "" {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
		var m tgbotapi.Message
		m, err = bot.Send(msg)
		if err != nil {
			log.Printf("Error sending reply to command: %v", err)
			return
		}
		deleteConfig := tgbotapi.DeleteMessageConfig{
			ChatID:    m.Chat.ID,
			MessageID: m.MessageID,
		}
		go messageDeleter(bot, deleteConfig, appConfig.AnswersTimeToDelete)

	}
	return
}

func questionCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	q, err := parseSlashQuestion(m)
	if err != nil {
		log.Printf("Unvalid command format\n")
		reply = "Неверный формат команды"
		return
	}
	questionID, err := store.addQuestion(q)
	if err != nil {
		log.Printf("Error while adding question : %v\n", err)
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = fmt.Sprintf("Вопрос принят, его id: %d", questionID)
	return
}

func questionToCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	q, err := parseSlashQuestionTo(m)
	if err != nil {
		log.Printf("Error while parsing message")
		reply = "Неправильный формат"
		return
	}
	questionID, err := store.addQuestion(q)
	if err != nil {
		log.Printf("Error while adding question : %v\n", err)
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = fmt.Sprintf("Вопрос принят, его id: %d", questionID)
	return
}

func closeCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	qID, err := parseSlashClose(m)
	if err != nil {
		log.Print(err)
		reply = "Неверный формат команды"
		return
	}

	question, err := store.getQuestion(qID)
	if err != nil {
		if err == QuestionDoesntExist {
			log.Println(err)
			reply = "Вопрос с таким id не существует"
			return
		} else {
			log.Println(err)
			reply = "Ошибка доступа к базе данных"
			return
		}
	}

	if m.From.UserName != question.User && !(inGroup(appConfig.Admins, m.From.UserName)) {
		err = NotEnoughPermissions
		reply = "Недостаточно прав"
		return
	}

	err = store.closeQuestion(qID)
	if err != nil {
		log.Print(err)
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = "Вопрос закрыт"
	return
}

func openCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	qID, err := parseSlashOpen(m)
	if err != nil {
		log.Print(err)
		reply = "Неверный формат команды"
		return
	}

	question, err := store.getQuestion(qID)
	if m.From.UserName != question.User && !(inGroup(appConfig.Admins, m.From.UserName)) {
		err = NotEnoughPermissions
		reply = "Недостаточно прав"
		return
	}

	err = store.openQuestion(qID)
	if err != nil {
		log.Print(err)
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = "Вопрос открыт"
	return
}

func listToMeQuestionsCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	questions, err := store.findAllQuestionsTo(m.From.UserName)
	if err != nil {
		log.Printf("Error while accessing questiong: %v\n", err)
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = listQuestions(questions)
	return
}

func listQuestionsCommandExec(store *SQLStore) (reply string, err error) {
	questions, err := store.findAllQuestionsTo(AllGroupName)
	if err != nil {
		log.Printf("Error with list_questions : %v\n", err)
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = listQuestions(questions)
	return
}

func answerCommandExec(m *tgbotapi.Message, store *SQLStore, bot *tgbotapi.BotAPI) (reply string, err error) {
	answer, err := parseSlashAnswer(m)
	if err != nil {
		if err == WrongCommandFormat {
			reply = "Неправильный формат команды"
		} else {
			reply = "Неправильный id вопроса"
		}
		return
	}

	question, err := store.getQuestion(answer.QuestionID)
	if err != nil {
		log.Printf("Error in getting question by id %v\n", err)
		if err == QuestionDoesntExist {
			reply = "Вопрос с таким id отсутствует в базе данных"
		} else {
			reply = "Ошибка доступа к базе данных"
		}
		return
	}

	answerID, err := store.addAnswer(answer)
	if err != nil {
		reply = "Ошибка доступа к базе данных"
		return
	}

	if question.Rec.User != AllGroupName {
		// answer by Receiver automatically closes question
		err = store.closeQuestion(question.QuestionID)
		if err != nil {
			log.Println(err)
			reply = "Ошибка доступа к базе данных"
			return
		}
	}

	if question.ChatID != m.Chat.ID {
		log.Println("Making asker notification")
		msg := makeAskerNotification(answer, question)
		var m tgbotapi.Message
		m, err = bot.Send(msg)
		deleteConfig := tgbotapi.DeleteMessageConfig{
			ChatID:    m.Chat.ID,
			MessageID: m.MessageID,
		}
		go messageDeleter(bot, deleteConfig, appConfig.NotificationsTimeToDelete)

		if err != nil {
			log.Println(err)
		}
	}

	reply = fmt.Sprintf("Ответ сохранен, его id: %d", answerID)
	return
}

func listAnswersCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	questionID, err := parseSlashListAnswers(m)
	if err != nil {
		reply = "Неверный формат команды"
		return
	}
	question, err := store.getQuestion(questionID)
	if err != nil {
		if err == QuestionDoesntExist {
			reply = "Вопроса с таким id нет в базе данных"
		} else {
			reply = "Ошибка доступа к базе данных"
		}
		return
	}
	answers, err := store.findAllAnswersFor(questionID)
	if err != nil {
		reply = "Ошибка доступа к базе данных"
		return
	}
	reply = listAnswers(question, answers)
	return
}

func deleteAnswerCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	answerID, err := parseSlashDeleteAnswer(m)
	if err != nil {
		reply = "Неверный формат команды"
		return
	}
	answer, err := store.getAnswer(answerID)
	if err != nil {
		if err == AnswerDoesntExist {
			reply = "Вопроса с таким id нет в базе данных"
		} else {
			reply = "Ошибка доступа к базе данных"
		}
		return
	}
	if answer.User != m.From.UserName && !(inGroup(appConfig.Admins, m.From.UserName)) {
		reply = "Недостаточно прав"
		err = NotEnoughPermissions
		return
	}
	err = store.deleteAnswer(answerID)
	if err != nil {
		reply = "Ошибка доступа к базе данных"
		return
	}
	return
}

func deleteQuestionCommandExec(m *tgbotapi.Message, store *SQLStore) (reply string, err error) {
	questionID, err := parseSlashDeleteQuestion(m)
	if err != nil {
		reply = "Неверный формат команды"
		return
	}
	question, err := store.getQuestion(questionID)
	if err != nil {
		if err == QuestionDoesntExist {
			reply = "Вопроса с таким id нет в базе данных"
		} else {
			reply = "Ошибка доступа к базе данных"
		}
		return
	}

	if question.User != m.From.UserName && !(inGroup(appConfig.Admins, m.From.UserName)) {
		err = NotEnoughPermissions
		return
	}
	err = store.deleteQuestion(questionID)

	if err != nil {
		return
	}
	return
}

func commandExec(bot *tgbotapi.BotAPI, update *tgbotapi.Update, store *SQLStore) (reply string, err error) {
	switch update.Message.Command() {
	case "start":
		reply = getStartReply()
	case "close":
		reply, err = closeCommandExec(update.Message, store)
		if err != nil {
			break
		}
	case "open":
		reply, err = openCommandExec(update.Message, store)
		if err != nil {
			break
		}
	case "question":
		reply, err = questionCommandExec(update.Message, store)
		if err != nil {
			break
		}
	case "question_to":
		reply, err = questionToCommandExec(update.Message, store)
		if err != nil {
			break
		}
	case "list_questions":
		reply, err = listQuestionsCommandExec(store)
		if err != nil {
			break
		}
	case "list_questions_to_me":
		reply, err = listToMeQuestionsCommandExec(update.Message, store)
		if err != nil {
			break
		}
	case "answer":
		reply, err = answerCommandExec(update.Message, store, bot)
		if err != nil {
			break
		}
	case "list_answers":
		reply, err = listAnswersCommandExec(update.Message, store)
		if err != nil {
			break
		}
	case "delete_answer":
		reply, err = deleteAnswerCommandExec(update.Message, store)
		if err != nil {
			log.Println(err)
			break
		}
	case "delete_question":
		reply, err = deleteQuestionCommandExec(update.Message, store)
		if err != nil {
			log.Println(err)
			break
		}
	case "list_my_answers":
		reply = "Command is not implemented yet"
	case "list_my_questions":
		reply = "Command is not implemented yet"
	case "important":
		reply = "Command is not implemented yet"
	case "list_important":
		reply = "Command is not implemented yet"
	case "delete_important":
		reply = "Command is not implemented yet"
	default:
		log.Printf("%v is uknown command\n", update.Message.Command())
		err = UknownCommand
		reply = "Неверная команда"
	}
	return
}

func makeAskerNotification(answer *Answer, question *Question) (msg tgbotapi.MessageConfig) {
	message_text := fmt.Sprintf(
		"На вопрос [%d], заданный @%s:\n        \"%s\"\n появился ответ от @%s:\n        \"%s\"",
		question.QuestionID, question.User, question.Text, answer.User, answer.Text)
	msg = tgbotapi.NewMessage(question.ChatID, message_text)
	return
}

func getStartReply() (reply string) {
	reply = "Привет, я телеграм бот, созданный для управления чатами групп ФББ"
	return
}

func listQuestions(lst QuestionList) (info string) {
	if len(lst) == 0 {
		info = "Нет вопросов"
		return
	}
	questionsInfo := []string{}
	for _, q := range lst {
		info := fmt.Sprintf("[%d] @%s cпросил в  %v:\n    %s", q.QuestionID, q.User, q.Date, q.Text)
		questionsInfo = append(questionsInfo, info)
	}
	info = strings.Join(questionsInfo, "\n")
	return
}

func listAnswers(q *Question, lst []*Answer) (info string) {
	if len(lst) == 0 {
		info = "Нет ответов"
		return
	}
	questionStr := fmt.Sprintf("На ваш вопрос от %v:\n    :%s\n",
		q.Date, q.Text)

	answersInfo := make([]string, len(lst))
	for ind, a := range lst {
		infStr := fmt.Sprintf("@%s ответил в %v:\n    %s", a.User, a.Date, a.Text)
		answersInfo[ind] = infStr
	}
	info = questionStr + strings.Join(answersInfo, "\n")
	return
}