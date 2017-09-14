package main

import (
	"database/sql"
	"sync"
	"time"
)

type QuestionList []*Question

type Message interface {
	GetHash() string
}

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

func (q *Question) GetHash() string {
	return GetMD5Hash(q.Text + "|" + q.User)
}

type Answer struct {
	User       string
	Text       string
	Date       time.Time
	QuestionID int
	AnswerID   int
}

func (a *Answer) GetHash() string {
	return GetMD5Hash(a.Text + "|" + a.User)
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
	InlineAnswersTimeToDelete int
	NotificationsTimeToDelete int
}

type TempMessage struct {
	Message Message // interface actually is a pointer, so there is no need to store it as pointer
	Tag     string
	Time    int64
}

type MessagePull struct {
	sync.Mutex
	cleanInterval int
	storeTime     int
	messages      chan *TempMessage
	stop          chan struct{}
	get           chan string
	outMessages   chan Message
	delete chan string // delete message immediately
}
