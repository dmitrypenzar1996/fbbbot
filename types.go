package main

import (
	"database/sql"
	"sync"
	"time"
)

type QuestionList []*Question

type Question struct {
	User       string
	Text       string
	Date       time.Time
	Rec        *Receiver
	Answers    []*Answer
	IsClosed   bool
	ChatID     int64
	QuestionID int
}

type Answer struct {
	User       string
	Text       string
	Date       time.Time
	QuestionID int
	AnswerID   int
}

type Receiver struct {
	User string
}

func NewReceiver(user string) *Receiver {
	return &Receiver{user}
}

type SQLStore struct {
	db   *sql.DB
	path string
	sync.Mutex
}

type Note struct {
	NoteID int
	User   string
	Text   string
	Date   time.Time
}

type AppConfig struct {
	TelegramBotToken          string
	Admins                    []string
	ErrorsTimeToDelete        int
	CommandsTimeToDelete      int
	InlineAnswersTimeToDelete       int
	NotificationsTimeToDelete int
}
