package rabbitmq

import (
	"encoding/json"
	"fmt"
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
	ch   *amqp.Channel
	q    amqp.Queue
}

// Message for producing to queue
type Message struct {
	Title   string
	Url     string
	Payload string
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
		ch:   ch,
		q:    q,
	}
	return queue, queue.close, nil
}

func (q *Queue) Produce(m *Message) error {
	body, err := json.Marshal(&m)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	err = q.ch.Publish("", q.q.Name, false, false, amqp.Publishing{
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
	err := q.ch.Close()
	if err != nil {
		return fmt.Errorf("failed to close channel: %w", err)
	}
	err = q.conn.Close()
	if err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}
	return nil
}
