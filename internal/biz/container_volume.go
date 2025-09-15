package biz

import (
	"github.com/coder-lulu/nb-agent/internal/http/request"
	"github.com/coder-lulu/nb-agent/pkg/types"
)

type ContainerVolumeRepo interface {
	List() ([]types.ContainerVolume, error)
	Create(req *request.ContainerVolumeCreate) (string, error)
	Remove(id string) error
	Prune() error
}
