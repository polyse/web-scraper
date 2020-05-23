package locker

import (
	"time"

	"github.com/mediocregopher/radix/v3"
)

type Config struct {
	Network string
	Addr    string
	Pass    string
	Size    int
}

type Conn struct {
	p *radix.Pool
}

func NewConn(c *Config) (*Conn, func() error, error) {
	connectionPath := func(network, addr string) (radix.Conn, error) {
		var opts []radix.DialOpt
		if c.Pass != "" {
			opts = append(opts, radix.DialAuthPass(c.Pass))
		}
		return radix.Dial(network, addr, opts...)
	}

	pool, err := radix.NewPool(c.Network, c.Addr, c.Size, radix.PoolConnFunc(connectionPath))
	return &Conn{p: pool}, func() error {
		return pool.Close()
	}, err
}

func (c *Conn) TryLock(url string) (bool, error) {
	var res string
	err := c.p.Do(radix.Cmd(&res, "SET", url, time.Now().String(), "NX"))
	return res == "OK", err
}

func (c *Conn) Exp(url string, seconds int) (bool, error) {
	var res int
	err := c.p.Do(radix.FlatCmd(&res, "EXPIRE", url, seconds))
	return res == 1, err
}

func (c *Conn) Unlock(url string) (bool, error) {
	var res int
	err := c.p.Do(radix.Cmd(&res, "DEL", url))
	return res == 1, err
}
