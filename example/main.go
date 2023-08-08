package main

import (
	"github.com/demouth/hankyo"
)

func main() {
	h := hankyo.New()
	h.Get("/ping", func(c *hankyo.Context) {
		c.Response.Write([]byte("pong"))
	})
	h.Get("/greeting", func(c *hankyo.Context) {
		c.Response.Write([]byte("hello"))
	})
	h.Run(":8080")
}
