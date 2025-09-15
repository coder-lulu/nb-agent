package biz

import (
	"github.com/coder-lulu/nb-agent/internal/http/request"
	"github.com/coder-lulu/nb-agent/pkg/types"
)

type ContainerNetworkRepo interface {
	List() ([]types.ContainerNetwork, error)
	Create(req *request.ContainerNetworkCreate) (string, error)
	Remove(id string) error
	Prune() error
}
