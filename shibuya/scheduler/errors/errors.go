package errors

import (
	"errors"
	"fmt"
)

var (
	IngressError = errors.New("Error with Ingress-")
)

func MakeSchedulerIngressError(err error) error {
	return fmt.Errorf("%w%s", IngressError, err.Error())
}

func MakeIPNotAssignedError() error {
	return fmt.Errorf("%w%s", IngressError, "IP is not assigned yet")
}

type NoResourcesFoundErr struct {
	Err     error
	Message string
}

func (e *NoResourcesFoundErr) Error() string {
	return e.Message
}
