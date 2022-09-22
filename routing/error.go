package routing

import (
	"fmt"

	"github.com/ipfs/kubo/config"
)

type ParamNeededError struct {
	ParamName  string
	RouterType config.RouterType
}

func NewParamNeededErr(param string, routing config.RouterType) error {
	return &ParamNeededError{
		ParamName:  param,
		RouterType: routing,
	}
}

func (e *ParamNeededError) Error() string {
	return fmt.Sprintf("configuration param '%v' is needed for %v delegated routing types", e.ParamName, e.RouterType)
}
