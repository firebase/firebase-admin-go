// Package apps is the entry point to the Firebase Admin SDK. It provides functionality for initializing and managing
// App instances, which serve as central entities that provide access to various other Firebase services exposed from
// the SDK.
package apps

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/firebase/firebase-admin-go/credentials"
	"github.com/firebase/firebase-admin-go/internal"
)

const defaultName string = "[DEFAULT]"

var apps = make(map[string]App)
var mutex = &sync.Mutex{}

// An App holds configuration and state common to all Firebase services that are exposed from the SDK.
//
// Client code should initialize an App with a valid authentication credential, and then use it to access
// the necessary Firebase services.
type App interface {
	// Name returns the name of this App.
	Name() string

	// Credential returns the credential used tp initialize this App.
	Credential() credentials.Credential

	// Auth returns an instance of the auth.Auth service.
	//
	// Multiple calls to Auth may return the same value. Auth panics if the App is already deleted.
	// Auth() auth.Auth

	// Del gracefully terminates and deletes this App.
	//
	// Trying to obtain a Firebase service from the App after a call to Del will panic.
	Del()
}

// Conf represents the configuration used to initialize an App.
type Conf struct {
	Name string
	Cred credentials.Credential
}

type appImpl struct {
	Ctx     *internal.Context
	Mutex   *sync.Mutex
	Serv    map[string]internal.AppService
	Deleted bool
}

func (a *appImpl) Name() string {
	return a.Ctx.Name
}

func (a *appImpl) Credential() credentials.Credential {
	return a.Ctx.Cred
}

func (a *appImpl) Del() {
	mutex.Lock()
	defer mutex.Unlock()
	a.Mutex.Lock()
	defer a.Mutex.Unlock()

	if a.Deleted {
		return
	}
	if _, exists := apps[a.Name()]; exists {
		delete(apps, a.Name())
	}

	for _, s := range a.Serv {
		s.Del()
	}
	a.Serv = nil
	a.Deleted = true
}

func (a *appImpl) service(id string, fn func() interface{}) interface{} {
	a.Mutex.Lock()
	defer a.Mutex.Unlock()
	if a.Deleted {
		var msg string
		if a.Name() == defaultName {
			msg = "Default app is deleted."
		} else {
			msg = fmt.Sprintf("App '%s' is deleted.", a.Name())
		}
		panic(msg)
	}

	var s interface{}
	var ok bool
	if s, ok = a.Serv[id]; !ok {
		s = fn()
		a.Serv[id] = s.(internal.AppService)
	}
	return s
}

// New initializes a new Firebase App using the specified configuration. New returns an error if
// the given configuration is invalid, or if an App by the same name already exists.
func New(c *Conf) (App, error) {
	mutex.Lock()
	defer mutex.Unlock()

	var name string
	var cred credentials.Credential

	if c == nil || c.Name == "" {
		name = defaultName
	} else {
		name = c.Name
	}

	if c == nil || c.Cred == nil {
		var err error
		if cred, err = credentials.NewAppDefault(context.Background()); err != nil {
			return nil, err
		}
	} else {
		cred = c.Cred
	}

	if _, exists := apps[name]; exists {
		var msg string
		if name == defaultName {
			msg = "The default Firebase app already exists. This means you called app.New() multiple " +
				"times. If you want to initialize multiple apps, specify a unique name for each app " +
				"instance via the app.Options argument passed into app.New()."
		} else {
			msg = fmt.Sprintf("Firebase app named '%s' already exists. This means you called app.New() "+
				"multiple times with the same name option. Make sure to provide a unique name in the "+
				"app.Options each time you call app.New().", name)
		}
		return nil, errors.New(msg)
	}

	a := &appImpl{
		Ctx:   &internal.Context{Name: name, Cred: cred},
		Mutex: &sync.Mutex{},
		Serv:  make(map[string]internal.AppService),
	}
	apps[name] = a
	return a, nil
}

// Default returns the default Firebase App. If the default App is not initialized, Default returns an error.
func Default() (App, error) {
	return Get(defaultName)
}

// Get returns the Firebase App identified by the specified name. If the specified App is not initialized, Get
// returns an error.
func Get(name string) (App, error) {
	mutex.Lock()
	defer mutex.Unlock()
	if app, ok := apps[name]; ok {
		return app, nil
	}

	var msg string
	if name == defaultName {
		msg = "The default Firebase app does not exist. Make sure to initialize the SDK by calling app.New()."
	} else {
		msg = fmt.Sprintf("Firebase app named '%s' does not exist. Make sure to initialize the SDK by "+
			" calling app.New() with your app name in the app.Options argument.", name)
	}
	return nil, errors.New(msg)
}
