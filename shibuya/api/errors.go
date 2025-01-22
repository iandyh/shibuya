package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/rakutentech/shibuya/shibuya/model"
	serrors "github.com/rakutentech/shibuya/shibuya/scheduler/errors"
	log "github.com/sirupsen/logrus"
)

var (
	noPermissionErr   = errors.New("403-")
	invalidRequestErr = errors.New("400-")
	ServerErr         = errors.New("500-")
)

func makeLoginError() error {
	return fmt.Errorf("%wyou need to login", noPermissionErr)
}

func makeInvalidRequestError(message string) error {
	return fmt.Errorf("%w%s", invalidRequestErr, message)
}

func makeNoPermissionErr(message string) error {
	return fmt.Errorf("%w%s", noPermissionErr, message)
}

func makeInternalServerError(message string) error {
	return fmt.Errorf("%w%s", ServerErr, message)
}

// you don't have permission error can be put into func
// invalid id can be put into func
func makeInvalidResourceError(resource string) error {
	return fmt.Errorf("%winvalid %s", invalidRequestErr, resource)
}

func makeProjectOwnershipError() error {
	return fmt.Errorf("%w%s", noPermissionErr, "You don't own the project")
}

func makeCollectionOwnershipError() error {
	return fmt.Errorf("%w%s", noPermissionErr, "You don't own the collection")
}

func handleErrorsFromExt(w http.ResponseWriter, err error) error {
	var (
		dbe                   *model.DBError
		noResourcesFoundError *serrors.NoResourcesFoundErr
	)
	switch {
	case errors.As(err, &dbe):
		makeFailMessage(w, dbe.Error(), http.StatusNotFound)
		return nil
	case errors.As(err, &noResourcesFoundError):
		makeFailMessage(w, noResourcesFoundError.Message, http.StatusNotFound)
		return nil
	}
	return err
}

// handles errors from other packages, like model, scheduler, etc.
// unhandle errors will be returned
func handleErrors(w http.ResponseWriter, err error) {
	unhandledError := handleErrorsFromExt(w, err)
	if unhandledError != nil { // if unhandleError is not nil, it's the same as original error
		switch {
		case errors.Is(err, noPermissionErr):
			makeFailMessage(w, err.Error(), http.StatusForbidden)
		case errors.Is(err, invalidRequestErr):
			makeFailMessage(w, err.Error(), http.StatusBadRequest)
		default:
			log.Printf("api error: %v", err)
			makeFailMessage(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
