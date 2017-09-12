package main

import "errors"

var appConfig *AppConfig

var QuestionDoesntExist = errors.New("Question with such ID doesn't exist")
var AnswerDoesntExist = errors.New("Answer with such ID doesn't exist")
var WrongCommandFormat = errors.New("Wrong command format")
var NotEnoughPermissions = errors.New("User has not enough permission for that action")
var NoteDoesntExist = errors.New("Note with such ID doesn't exist")
var UknownCommand = errors.New("Unknown command")
var WrongChatID = errors.New("Wrong chat id")
var InlineCommands = []string{"list_questions", "list_answers", "list_questions_to_me", "list_answers_to_me",
      "question", "answer", "delete_answer", "delete_question"}

var MaxSendInlineObjects = 10

const appConfigPath string = "config.json"
const AllGroupName string = "all"
const BotMessageSign = "----------------------------------------"
const MaxShownMessageLength = 64
const EmptyMessage = "Пустое сообщение"
const NotExistsMessage = "Данной команды не существует"
const dateFormat = "Jan 2, 2006 в 3:04pm"
