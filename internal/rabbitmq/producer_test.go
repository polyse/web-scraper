package rabbitmq

import (
	"encoding/json"
	"reflect"
	"strconv"
	"sync"
	"testing"
)

func TestQueue_Produce(t *testing.T) {
	// connect to rabbitmq
	c, err := Connect(&Config{
		Host:      "localhost",
		Port:      "5672",
		QueueName: "test",
	})
	if err != nil {
		t.Fatal("Error on connection:", err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			t.Fatal("Error on close connection:", err)
		}
	}()

	// create consumer
	// WaitGroup for waiting all messages to deliver
	var wg sync.WaitGroup
	wg.Add(1)
	msgs, err := c.ch.Consume("test", "", false, false, false, false, nil)
	if err != nil {
		t.Fatal("Error on creating consumer:", err)
	}
	act := make([]Message, 10)
	consumed := 0
	go func() {
		for m := range msgs {
			// get message
			var message Message
			err := json.Unmarshal(m.Body, &message)
			if err != nil {
				t.Error("Error on consuming:", err)
			}
			t.Log("Consumed message:", message)
			// save it to slice
			index, err := strconv.Atoi(message.Payload)
			if err != nil {
				t.Error("Error on converting body to int:", err)
			}
			act[index] = message
			err = m.Ack(false)
			if err != nil {
				t.Error("Error on act:", err)
			}
			consumed++
			if consumed == 10 {
				wg.Done()
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
		if err != nil {
			t.Errorf("Error on producing %d: %s", i, err)
		}
		t.Log("Produced:", message)
		exp[i] = message
	}
	wg.Wait()

	// compare
	t.Log("Exp:", exp)
	t.Log("Act:", act)
	if !reflect.DeepEqual(act, exp) {
		t.Fatal("Exp != act")
	}
}
