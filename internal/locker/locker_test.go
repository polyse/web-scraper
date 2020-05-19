package locker

import (
	"testing"
	"time"

	"github.com/mediocregopher/radix/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testConfig = Config{
	Network: "tcp",
	Addr:    ":6379",
	Size:    1,
}

func TestNewConn(t *testing.T) {
	conn, closer, err := NewConn(&testConfig)
	require.NoError(t, err, "Error on connection")
	defer func() {
		err = closer()
		require.NoError(t, err, "Error on close")
	}()
	var act string
	err = conn.p.Do(radix.Cmd(&act, "PING"))
	require.NoError(t, err, "Error on PING")
	assert.Equal(t, "PONG", act)
}

func TestConn_TryLock(t *testing.T) {
	conn, closer, err := NewConn(&testConfig)
	require.NoError(t, err, "Error on connection")
	defer func() {
		err = closer()
		require.NoError(t, err, "Error on close")
	}()
	// try-lock first time
	act, err := conn.TryLock("test-url")
	require.NoError(t, err, "Error on try-lock 1")
	require.Equal(t, true, act)
	// try-lock again, expect fail
	act, err = conn.TryLock("test-url")
	require.NoError(t, err, "Error on try-lock 2")
	require.Equal(t, false, act)
	// remove key
	err = conn.p.Do(radix.Cmd(nil, "DEL", "test-url"))
	require.NoError(t, err, "Error on deleting key 'test-url'")
}

func TestConn_Unlock(t *testing.T) {
	conn, closer, err := NewConn(&testConfig)
	require.NoError(t, err, "Error on connection")
	defer func() {
		err = closer()
		require.NoError(t, err, "Error on close")
	}()
	// try-lock
	act, err := conn.TryLock("test-url")
	require.NoError(t, err, "Error on try-lock 1")
	require.Equal(t, true, act)
	// unlock 1 time
	act, err = conn.Unlock("test-url")
	require.NoError(t, err, "Error on unlock 1")
	require.Equal(t, true, act)
	// unlock 2 time
	act, err = conn.Unlock("test-url")
	require.NoError(t, err, "Error on unlock 2")
	require.Equal(t, false, act)
}

func TestConn_Exp(t *testing.T) {
	conn, closer, err := NewConn(&testConfig)
	require.NoError(t, err, "Error on connection")
	defer func() {
		err = conn.p.Do(radix.Cmd(nil, "DEL", "test-url"))
		require.NoError(t, err, "Error on deleting 'test-url' in defer")
		err = closer()
		require.NoError(t, err, "Error on close")
	}()
	// try-lock
	act, err := conn.TryLock("test-url")
	require.NoError(t, err, "Error on try-lock 1")
	require.Equal(t, true, act)
	// set exp
	act, err = conn.Exp("test-url", 2)
	require.NoError(t, err, "Error on try-lock 1")
	require.Equal(t, true, act)
	// wait for 2 seconds
	time.Sleep(time.Second * 2)
	// try-lock again
	act, err = conn.TryLock("test-url")
	require.NoError(t, err, "Error on try-lock 2")
	require.Equal(t, true, act)
}
