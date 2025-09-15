package request

import "github.com/coder-lulu/nb-agent/pkg/types"

type ContainerComposeGet struct {
	Name string `uri:"name" validate:"required|regex:^[a-zA-Z0-9_-]+$"`
}

type ContainerComposeCreate struct {
	Name    string     `json:"name" validate:"required|regex:^[a-zA-Z0-9_-]+$"`
	Compose string     `json:"compose" validate:"required"`
	Envs    []types.KV `json:"envs"`
}

type ContainerComposeUpdate struct {
	Name    string     `uri:"name" validate:"required|regex:^[a-zA-Z0-9_-]+$"`
	Compose string     `json:"compose" validate:"required"`
	Envs    []types.KV `json:"envs"`
}

type ContainerComposeUp struct {
	Name  string `uri:"name" validate:"required|regex:^[a-zA-Z0-9_-]+$"`
	Force bool   `json:"force"`
}

type ContainerComposeDown struct {
	Name string `uri:"name" validate:"required|regex:^[a-zA-Z0-9_-]+$"`
}

type ContainerComposeRemove struct {
	Name string `uri:"name" validate:"required|regex:^[a-zA-Z0-9_-]+$"`
}
