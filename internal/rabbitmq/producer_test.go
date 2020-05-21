package rabbitmq

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	sdk "github.com/polyse/database-sdk"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	msgs, err := c.Ch.Consume("test", "", false, false, false, false, nil)
	require.NoError(t, err, "Error on creating consumer")
	act := make([]sdk.RawData, 10)
	consumed := 0
	quit := make(chan struct{})
	go func() {
		time.Sleep(time.Duration(5 * time.Second))
		t.Log("Start listen")
		for {
			select {
			case <-quit:
				return
			case m := <-msgs:
				// get message
				var message sdk.RawData
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
	exp := make([]sdk.RawData, 10)
	for i := 0; i < 10; i++ {
		message := sdk.RawData{
			Source: sdk.Source{
				Date:  time.Time{},
				Title: fmt.Sprintf("Title %v", i),
			},
			Url:  fmt.Sprintf("Url %v", i),
			Data: strconv.Itoa(i),
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
	require.ElementsMatch(t, act, exp)
}
