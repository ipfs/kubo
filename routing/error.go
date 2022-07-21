package routing

import "fmt"

type ParamNeededError struct {
	ParamName  string
	RouterType string
}

func NewParamNeededErr(param, routing string) error {
	return &ParamNeededError{
		ParamName:  param,
		RouterType: routing,
	}
}

func (e *ParamNeededError) Error() string {
	return fmt.Sprintf("configuration param '%v' is needed for %v delegated routing types", e.ParamName, e.RouterType)
}

type RouterTypeNotFoundError struct {
	RouterType string
}

func (e *RouterTypeNotFoundError) Error() string {
	return fmt.Sprintf("router type %v is not supported", e.RouterType)
}
