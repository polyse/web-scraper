package rabbitmq

import (
	"encoding/json"
	"fmt"

	database_sdk "github.com/polyse/database-sdk"
	"github.com/streadway/amqp"
)

// Config for connecting to RabbitMq
type Config struct {
	Uri       string
	QueueName string
}

// Queue holds a connection and opened channel with queue
type Queue struct {
	conn *amqp.Connection
	Ch   *amqp.Channel
	q    amqp.Queue
}

func Connect(c *Config) (*Queue, func() error, error) {
	conn, err := amqp.Dial(c.Uri)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open connection: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open channel: %w", err)
	}
	q, err := ch.QueueDeclare(c.QueueName, true, false, false, false, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to declare a queue: %w", err)
	}
	queue := &Queue{
		conn: conn,
		Ch:   ch,
		q:    q,
	}
	return queue, queue.close, nil
}

func (q *Queue) Produce(rawData *database_sdk.RawData) error {
	body, err := json.Marshal(&rawData)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	err = q.Ch.Publish("", q.q.Name, false, false, amqp.Publishing{
		ContentType:     "application/json",
		ContentEncoding: "UTF-8",
		Body:            body,
	})
	if err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}
	return nil
}

func (q *Queue) close() error {
	err := q.Ch.Close()
	if err != nil {
		return fmt.Errorf("failed to close channel: %w", err)
	}
	err = q.conn.Close()
	if err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}
	return nil
}
