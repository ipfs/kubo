package routing

import (
	"fmt"
	"strings"

	"github.com/ipfs/kubo/config"
)

type ParamNeededError struct {
	ParamName  config.RouterParam
	RouterType config.RouterType
}

func NewParamNeededErr(param config.RouterParam, routing config.RouterType) error {
	return &ParamNeededError{
		ParamName:  param,
		RouterType: routing,
	}
}

func (e *ParamNeededError) Error() string {
	return fmt.Sprintf("configuration param '%v' is needed for %v delegated routing types", e.ParamName, e.RouterType)
}

type RouterTypeNotFoundError struct {
	RouterType config.RouterType
}

func (e *RouterTypeNotFoundError) Error() string {
	return fmt.Sprintf("router type %v is not supported", e.RouterType)
}

type InvalidValueError struct {
	ParamName    config.RouterParam
	InvalidValue string
	ValidValues  []string
}

func (e *InvalidValueError) Error() string {
	return fmt.Sprintf(
		"value `%s` for configuration param `%s` is not valid. Valid values: %s",
		e.InvalidValue, e.ParamName, strings.Join(e.ValidValues, ", "),
	)
}
