package tools

import (
	"strings"

	"github.com/afonp/microvault/internal/db"
	"github.com/afonp/microvault/internal/hashing"
)

type Context struct {
	DBPath   string
	Volumes  string
	Replicas int
}

func (c *Context) GetRing() *hashing.Ring {
	ring := hashing.NewRing(c.Replicas)
	for _, v := range strings.Split(c.Volumes, ",") {
		ring.AddNode(strings.TrimSpace(v))
	}
	return ring
}

func (c *Context) GetStore() (*db.Store, error) {
	return db.NewStore(c.DBPath)
}
