package bootstrap

import (
	"github.com/coder-lulu/nb-agent/pkg/queue"
)

func NewQueue() *queue.Queue {
	return queue.New(100)
}
