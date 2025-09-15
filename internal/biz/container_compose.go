package biz

import "github.com/coder-lulu/nb-agent/pkg/types"

type ContainerComposeRepo interface {
	List() ([]types.ContainerCompose, error)
	Get(name string) (string, []types.KV, error)
	Create(name, compose string, envs []types.KV) error
	Update(name, compose string, envs []types.KV) error
	Up(name string, force bool) error
	Down(name string) error
	Remove(name string) error
}
