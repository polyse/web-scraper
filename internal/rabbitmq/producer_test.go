package rabbitmq

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strconv"
	"sync"
	"testing"
)

func TestQueue_Produce(t *testing.T) {
	// connect to rabbitmq
	c, closeConn, err := Connect(&Config{
		Uri:       "amqp://localhost:5672",
		QueueName: "test",
	})
	require.NoError(t, err, "Error on connection")
	defer func() {
		err := closeConn()
		require.NoError(t, err, "Error on close connection")
	}()

	// create consumer
	// WaitGroup for waiting all messages to deliver
	var wg sync.WaitGroup
	wg.Add(1)
	msgs, err := c.ch.Consume("test", "", false, false, false, false, nil)
	require.NoError(t, err, "Error on creating consumer")
	act := make([]Message, 10)
	consumed := 0
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-quit:
				return
			case m := <-msgs:
				// get message
				var message Message
				err := json.Unmarshal(m.Body, &message)
				if assert.NoError(t, err, "Error on consuming") {
					t.Log("Consumed message:", message)
					// save it to slice
					act[consumed] = message
					err = m.Ack(false)
					assert.NoError(t, err, "Error on act")
				}
				// count even on failed consume
				consumed++
				if consumed == 10 {
					wg.Done()
				}
			}
		}
	}()

	// publish messages
	exp := make([]Message, 10)
	for i := 0; i < 10; i++ {
		message := Message{
			Title:   "Title",
			Url:     "Url",
			Payload: strconv.Itoa(i),
		}
		err = c.Produce(&message)
		if assert.NoError(t, err, "Error on producing %d: %s", i, err) {
			t.Log("Produced:", message)
		}
		exp[i] = message
	}
	wg.Wait()
	close(quit)

	// compare
	t.Log(exp)
	t.Log(act)
	require.ElementsMatch(t, act, exp)
}
