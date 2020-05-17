package broker

import (
	"encoding/json"
	"reflect"
	"time"

	database_sdk "github.com/polyse/database-sdk"
	"github.com/polyse/web-scraper/internal/rabbitmq"
	zl "github.com/rs/zerolog/log"
	"github.com/streadway/amqp"
)

type Broker struct {
	q              *rabbitmq.Queue
	url            string
	numDoc         int
	timeout        time.Duration
	quit           chan struct{}
	dbClient       *database_sdk.DBClient
	collectionName string
	queueName      string
}

func NewBroker(q *rabbitmq.Queue, url, collectionName, queueName string, numDoc, timeout int, client *database_sdk.DBClient) *Broker {
	return &Broker{
		q:              q,
		url:            url,
		numDoc:         numDoc,
		timeout:        time.Duration(timeout),
		quit:           make(chan struct{}),
		dbClient:       client,
		collectionName: collectionName,
		queueName:      queueName,
	}
}

func (b *Broker) StartBroker() {
	dataCh, err := b.q.Ch.Consume(
		b.queueName, // queue
		"",          // consumer
		true,        // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		zl.Warn().Err(err)
		return
	}
	go b.Listener(dataCh)
}

func (b *Broker) SaveMessage(messages database_sdk.Documents) ([]rabbitmq.Message, int) {
	insertDocs, err := b.dbClient.SaveData(b.collectionName, messages)
	if err != nil {
		zl.Debug().Err(err).Msg("Something going wrong")
		return nil, len(messages.Documents)
	}
	remainingDocs := []rabbitmq.Message{}
	for _, insertDoc := range insertDocs.Documents {
		flag := false
		for _, message := range messages.Documents {
			if reflect.DeepEqual(insertDoc, message) == true {
				flag = true
				break
			}
		}
		if flag == false {
			remainingDocs = append(remainingDocs, rabbitmq.Message{
				Source: rabbitmq.Source{
					Date:  &insertDoc.Source.Date,
					Title: insertDoc.Source.Title,
				},
				Url:  insertDoc.Url,
				Data: insertDoc.Data,
			})
		}
	}
	return remainingDocs, len(messages.Documents) - len(remainingDocs)
}

func (b *Broker) Save(messages database_sdk.Documents, d amqp.Delivery) int {
	// send data to db
	remainingDocs, remainingCount := b.SaveMessage(messages)
	err := d.Ack(false)
	if err != nil {
		zl.Warn().Err(err).Msgf("Can't ack messages")
	}
	// if remaining, send back
	if remainingCount > 0 {
		for _, remainingDoc := range remainingDocs {
			err = b.q.Produce(&remainingDoc)
			if err != nil {
				zl.Warn().Err(err).Msg("Can't produce message")
			}
		}
	}
	return remainingCount
}

func (b *Broker) Listener(dataCh <-chan amqp.Delivery) {
	messages := database_sdk.Documents{}
	count := 0
	d := amqp.Delivery{}
	for {
		select {
		case d = <-dataCh:
			message := database_sdk.RawData{}
			err := json.Unmarshal(d.Body, message)
			if err != nil {
				zl.Warn().Err(err).Msg("Can't unmarshal doc")
			} else {
				count++
				messages.Documents = append(messages.Documents, message)
				if b.numDoc == count {
					_ = b.Save(messages, d)
				}
			}
		case <-time.After(b.timeout * time.Second):
			count = b.Save(messages, d)
		case <-b.quit:
			_ = b.Save(messages, d)
			return
		}
	}
}
