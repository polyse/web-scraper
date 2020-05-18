package main

import (
	"encoding/json"
	"os"
	"reflect"
	"time"

	database_sdk "github.com/polyse/database-sdk"
	"github.com/polyse/web-scraper/internal/rabbitmq"
	"github.com/rs/zerolog"
	zl "github.com/rs/zerolog/log"
	"github.com/streadway/amqp"
)

type Сonsumer struct {
	q              *rabbitmq.Queue
	url            string
	numDoc         int
	timeout        time.Duration
	quit           chan struct{}
	dbClient       *database_sdk.DBClient
	collectionName string
	queueName      string
}

func main() {
	cfg, err := newConfig()
	if err != nil {
		zl.Fatal().Msgf("Can't parse config")
	}

	logLevel, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		zl.Fatal().Err(err).Msgf("Can't parse loglevel")
	}
	zerolog.SetGlobalLevel(logLevel)
	zl.Logger = zl.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	q, closer, err := rabbitmq.Connect(&rabbitmq.Config{
		Uri:       cfg.RabbitmqUri,
		QueueName: cfg.QueueName,
	})

	if err != nil {
		zl.Fatal().Msgf("Can't connect to rmq")
	}
	defer closer()

	c := InitConsumer(cfg, q)
	c.StartConsume()
}

func InitConsumer(cfg *config, q *rabbitmq.Queue) *Сonsumer {
	newclient := database_sdk.NewDBClient(cfg.RabbitmqUri)
	return NewConsumer(q, cfg.RabbitmqUri, cfg.CollectionName, cfg.QueueName, cfg.NumDocument, cfg.Timeout, newclient)
}

func NewConsumer(q *rabbitmq.Queue, url, collectionName, queueName string, numDoc int, timeout time.Duration, client *database_sdk.DBClient) *Сonsumer {
	return &Сonsumer{
		q:              q,
		url:            url,
		numDoc:         numDoc,
		timeout:        timeout,
		quit:           make(chan struct{}),
		dbClient:       client,
		collectionName: collectionName,
		queueName:      queueName,
	}
}

func (b *Сonsumer) StartConsume() {
	dataCh, err := b.q.Ch.Consume(
		b.queueName, // queue
		"",          // consumer
		false,       // auto-ack
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

func (b *Сonsumer) SaveMessages(messages database_sdk.Documents, d []amqp.Delivery) {
	insertDocs, err := b.dbClient.SaveData(b.collectionName, messages)
	if err != nil {
		zl.Debug().Err(err).Msg("Can't save date to db")
		for i, _ := range messages.Documents {
			err := d[i].Nack(false, true)
			if err != nil {
				zl.Warn().Err(err).Msg("Can't send nack")
			}
		}
		return
	}
	for i, message := range messages.Documents {
		flag := false
		for _, insertDoc := range insertDocs.Documents {
			if reflect.DeepEqual(insertDoc.Url, message.Url) == true {
				flag = true
				err := d[i].Ack(false)
				if err != nil {
					zl.Warn().Err(err).Msgf("Can't ack messages")
				}
				break
			}
		}
		if flag == false {
			err := d[i].Nack(false, true)
			if err != nil {
				zl.Warn().Err(err).Msg("Can't send nack")
			}
		}
	}
}

func (b *Сonsumer) Listener(dataCh <-chan amqp.Delivery) {
	messages := database_sdk.Documents{}
	count := 0
	zl.Debug().Msg("Start listen")
	deliverys := []amqp.Delivery{}
	for {
		select {
		case d := <-dataCh:
			message := database_sdk.RawData{}
			err := json.Unmarshal(d.Body, message)
			if err != nil {
				zl.Warn().Err(err).Msg("Can't unmarshal doc")
				err := d.Nack(false, true)
				if err != nil {
					zl.Warn().Err(err).Msg("Can't send nack")
				}
			} else {
				count++
				messages.Documents = append(messages.Documents, message)
				deliverys = append(deliverys, d)
				if b.numDoc == count {
					b.SaveMessages(messages, deliverys)
					count = 0
					messages = database_sdk.Documents{}
				}
			}
		case <-time.After(b.timeout):
			b.SaveMessages(messages, deliverys)
			count = 0
			messages = database_sdk.Documents{}
		case <-b.quit:
			b.SaveMessages(messages, deliverys)
			return
		}
	}
}
