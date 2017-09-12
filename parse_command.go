package main

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strconv"
	"strings"
)

func parseSlashQuestion(m *tgbotapi.Message) (q *Question, err error) {
	q = new(Question)
	q.Date = m.Time().UTC()
	q.User = m.From.UserName
	q.Rec = NewReceiver(AllGroupName)
	q.Text = m.CommandArguments()
	if strings.TrimSpace(q.Text) == "" {
		err = WrongCommandFormat
		return
	}
	q.Answers = []*Answer{}
	q.IsClosed = false
	q.ChatID = m.Chat.ID
	q.QuestionID = -1
	return
}

func parseSlashQuestionTo(m *tgbotapi.Message) (q *Question, err error) {
	q = new(Question)
	q.Date = m.Time().UTC()
	q.User = m.From.UserName
	cmd_args := strings.SplitN(m.CommandArguments(), " ", 3)
	if len(cmd_args) != 3 {
		err = WrongCommandFormat
		return
	}
	q.Rec = NewReceiver(strings.Replace(cmd_args[0], "@", "", -1))
	q.Text = cmd_args[1]
	q.Answers = []*Answer{}
	q.IsClosed = false
	q.ChatID = m.Chat.ID
	q.QuestionID = -1
	return
}

func parseSlashDeleteAnswer(m *tgbotapi.Message) (answerID int, err error) {
	if m.CommandArguments() == "" {
		err = WrongCommandFormat
		return
	}
	answerID, err = strconv.Atoi(m.CommandArguments())
	if err != nil {
		return
	}
	return
}

func parseSlashDeleteQuestion(m *tgbotapi.Message) (questionID int, err error) {
	if m.CommandArguments() == "" {
		err = WrongCommandFormat
		return
	}
	questionID, err = strconv.Atoi(m.CommandArguments())
	if err != nil {
		return
	}
	return
}

func parseSlashListAnswers(m *tgbotapi.Message) (questionID int, err error) {
	questionStrID := m.CommandArguments()
	if questionStrID == "" {
		err = WrongCommandFormat
		return
	}
	questionID, err = strconv.Atoi(questionStrID)
	if err != nil {
		return
	}
	return
}

func parseSlashAnswer(m *tgbotapi.Message) (answer *Answer, err error) {
	cmd_args := strings.SplitN(m.CommandArguments(), " ", 2)
	if len(cmd_args) < 2 {
		err = WrongCommandFormat
		return
	}
	log.Println(cmd_args)
	quest_id, err := strconv.Atoi(cmd_args[0])
	if err != nil {
		return
	}
	answer = &Answer{
		Text:       cmd_args[1],
		User:       m.From.UserName,
		QuestionID: quest_id,
		Date:       m.Time(),
	}
	return
}

func parseSlashClose(m *tgbotapi.Message) (qID int, err error) {
	if m.CommandArguments() == "" {
		err = WrongCommandFormat
		return
	}
	qID, err = strconv.Atoi(m.CommandArguments())
	if err != nil {
		err = WrongCommandFormat
		return
	}
	return
}

func parseSlashOpen(m *tgbotapi.Message) (qID int, err error) {
	if m.CommandArguments() == "" {
		err = WrongCommandFormat
		return
	}
	qID, err = strconv.Atoi(m.CommandArguments())
	if err != nil {
		err = WrongCommandFormat
		return
	}
	return
}
