package consumer

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	sdk "github.com/polyse/database-sdk"
	"github.com/polyse/web-scraper/internal/rabbitmq"
	zl "github.com/rs/zerolog/log"
	"github.com/streadway/amqp"
)

type Consumer struct {
	q              *rabbitmq.Queue
	url            string
	numDoc         int
	timeout        time.Duration
	dbClient       *sdk.DBClient
	collectionName string
	queueName      string
	ctx            context.Context
	Wg             *sync.WaitGroup
}

type In struct {
	Url            string
	NumDoc         int
	Timeout        time.Duration
	CollectionName string
	QueueName      string
}

func NewConsumer(ctx context.Context, q *rabbitmq.Queue, in In, client *sdk.DBClient) *Consumer {
	return &Consumer{
		q:              q,
		url:            in.Url,
		numDoc:         in.NumDoc,
		timeout:        in.Timeout,
		dbClient:       client,
		collectionName: in.CollectionName,
		queueName:      in.QueueName,
		ctx:            ctx,
		Wg:             &sync.WaitGroup{},
	}
}

func (c *Consumer) StartConsume() error {
	dataCh, err := c.q.Ch.Consume(
		c.queueName, // queue
		"",          // Consumer
		false,       // auto-ack
		true,        // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return err
	}
	go c.listener(dataCh)
	return nil
}

func (c *Consumer) saveMessages(messages sdk.Documents, d []amqp.Delivery) {
	_, err := c.dbClient.SaveData(c.collectionName, messages)
	if err != nil {
		zl.Debug().Err(err).Msg("Can't save date to db")
		for i := range messages.Documents {
			err := d[i].Nack(false, true)
			if err != nil {
				zl.Warn().Err(err).Msg("Can't send nack")
			}
		}
		return
	}
	for i := range messages.Documents {
		err := d[i].Ack(false)
		if err != nil {
			zl.Warn().Err(err).Msgf("Can't ack messages")
		}
	}
}

func (c *Consumer) listener(dataCh <-chan amqp.Delivery) {
	defer c.Wg.Done()
	messages := sdk.Documents{}
	count := 0
	zl.Debug().Msg("Start listen")
	var deliveries []amqp.Delivery
	for {
		select {
		case d, ok := <-dataCh:
			if !ok {
				continue
			}
			message := sdk.RawData{}
			err := json.Unmarshal(d.Body, &message)
			if err != nil {
				zl.Warn().Err(err).Msg("Can't unmarshal doc")
				err := d.Nack(false, true)
				if err != nil {
					zl.Warn().Err(err).Msg("Can't send nack")
				}
				continue
			}
			count++
			zl.Debug().Msgf("Got message %v", count)
			messages.Documents = append(messages.Documents, message)
			deliveries = append(deliveries, d)
			if c.numDoc == count {
				zl.Info().Msgf("flush %d docs", c.numDoc)
				c.saveMessages(messages, deliveries)
				count = 0
				messages = sdk.Documents{}
				deliveries = []amqp.Delivery{}
			}
		case <-time.After(c.timeout):
			zl.Debug().Msg("Timeout end")
			if c.numDoc == 0 {
				continue
			}
			zl.Info().Msgf("flush %d docs", c.numDoc)
			c.saveMessages(messages, deliveries)
			count = 0
			messages = sdk.Documents{}
			deliveries = []amqp.Delivery{}
		case <-c.ctx.Done():
			zl.Debug().Msg("Finish")
			c.saveMessages(messages, deliveries)
			return
		}
	}
}
