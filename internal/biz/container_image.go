package biz

import (
	"github.com/coder-lulu/nb-agent/internal/http/request"
	"github.com/coder-lulu/nb-agent/pkg/types"
)

type ContainerImageRepo interface {
	List() ([]types.ContainerImage, error)
	Pull(req *request.ContainerImagePull) error
	Remove(id string) error
	Prune() error
}
