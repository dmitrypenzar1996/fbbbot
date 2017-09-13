package main

import (
	"log"
	"time"
)

func NewMessagePull(cleanInterval int, storeTime int) (p *MessagePull) {
	p = new(MessagePull)
	p.messages = make(chan *TempMessage)
	p.stop = make(chan struct{})
	p.cleanInterval = cleanInterval
	p.get = make(chan string)
	p.outMessages = make(chan Message)
	p.storeTime = storeTime
	return
}

func (p *MessagePull) init() {
	go p.storer()
}

func (p *MessagePull) addMessage(message Message) (tag string) {
	tag = message.GetHash()
	tempM := TempMessage{Message: message, Tag: tag,
		Time: time.Now().Unix(),
	}
	p.messages <- &tempM
	return
}

func (p *MessagePull) getMessage(tag string) (message Message, err error) {
	p.get <- tag
	message = <-p.outMessages
	if message == nil {
		err = QuestionDoesntExist
		return
	}
	return
}

func (p *MessagePull) storer() {
	ticker := time.NewTicker(time.Duration(p.cleanInterval) * time.Second)
	questionStore := make(map[string]*TempMessage)
	for {
		select {
		case q := <-p.messages:
			log.Printf("Add question to temporary storage")
			questionStore[q.Tag] = q
		case <-ticker.C:
			log.Println("Cleaning out-of-date temp messages")
			var deleteKeys []string
			for key, q := range questionStore {
				if time.Now().Unix()-q.Time > inlineTempQuestionStoreTime {
					deleteKeys = append(deleteKeys, key)
				}
			}

			for _, key := range deleteKeys {
				log.Printf("Removing question with tag %s", key)
				delete(questionStore, key)
			}
			log.Println("Done")
		case tag := <-p.get:
			if tempM, ok := questionStore[tag]; ok {
				p.outMessages <- tempM.Message
			} else {
				p.outMessages <- nil
			}

		case <-p.stop:
			return
		}
	}
}
