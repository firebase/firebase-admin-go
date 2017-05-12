// Package internal contains functionality that is only accessible from within the Admin SDK.
package internal

import (
	"github.com/firebase/firebase-admin-go/credentials"
)

// AppService represents a service initialized and managed by a Firebase App.
//
// Each Firebase service exposed from the Admin SDK should implement this interface. This enables the parent Firebase
// App to gracefully terminate Firebase services when they are no longer needed.
type AppService interface {
	// Del gracefully terminates this AppService by cleaning up any internal state, and releasing any resources
	// allocated.
	Del()
}

// AppConf represents the internal state of a Firebase App that is shared across all Firebase services.
type AppConf struct {
	Name string
	Cred credentials.Credential
}
