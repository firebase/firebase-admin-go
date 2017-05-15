// Package app is the entry point to the Firebase Admin SDK. It provides functionality for initializing and managing
// App instances, which serve as central entities that provide access to various other Firebase services exposed from
// the SDK.
package app

import (
	"errors"
	"fmt"
	"sync"

	"github.com/firebase/firebase-admin-go/credentials"
	"github.com/firebase/firebase-admin-go/internal"
)

const defaultName string = "[DEFAULT]"

// apps holds all Firebase Apps initialized using this SDK.
var apps = make(map[string]App)

// mutex guards access to package-level state variables (apps in particular)
var mutex = &sync.Mutex{}

// An App holds configuration and state common to all Firebase services that are exposed from the SDK.
//
// Client code should initialize an App with a valid authentication credential, and then use it to access
// Firebase services.
type App interface {
	// Name returns the name of this App.
	Name() string

	// Credential returns the credential used to initialize this App.
	Credential() credentials.Credential

	// Auth returns an instance of the auth.Auth service.
	//
	// Multiple calls to Auth may return the same value. Auth panics if the App is already deleted.
	// Auth() auth.Auth

	// Del gracefully terminates this App.
	//
	// Del stops any services associated with this App, and releases all allocated resources. Trying to obtain a
	// Firebase service from the App after a call to Del, or calling Del multiple times on an App will panic.
	Del()
}

// Conf represents the configuration used to initialize an App.
type Conf struct {
	Name string
	Cred credentials.Credential
}

type appImpl struct {
	Conf     *internal.AppConf
	Mutex    *sync.Mutex
	Services map[string]internal.AppService
	Deleted  bool
}

func (a *appImpl) Name() string {
	return a.Conf.Name
}

func (a *appImpl) Credential() credentials.Credential {
	return a.Conf.Cred
}

func (a *appImpl) Del() {
	mutex.Lock()
	defer mutex.Unlock()
	a.Mutex.Lock()
	defer a.Mutex.Unlock()

	a.checkNotDeleted()
	if _, exists := apps[a.Name()]; exists {
		delete(apps, a.Name())
	}

	for _, s := range a.Services {
		s.Del()
	}
	a.Services = nil
	a.Deleted = true
}

func (a *appImpl) checkNotDeleted() {
	if !a.Deleted {
		return
	}
	var msg string
	if a.Name() == defaultName {
		msg = "Default app is deleted."
	} else {
		msg = fmt.Sprintf("App %q is deleted.", a.Name())
	}
	panic(msg)
}

// service returns the AppService identified by the specified ID. If the AppService does not exist yet, the
// provided function is invoked to initialize a new instance.
func (a *appImpl) service(id string, fn func() internal.AppService) internal.AppService {
	a.Mutex.Lock()
	defer a.Mutex.Unlock()
	a.checkNotDeleted()

	var s internal.AppService
	var ok bool
	if s, ok = a.Services[id]; !ok {
		s = fn()
		a.Services[id] = s
	}
	return s
}

// New initializes a new Firebase App using the specified configuration. New returns an error if
// the given configuration is invalid, or if an App by the same name already exists.
func New(c *Conf) (App, error) {
	mutex.Lock()
	defer mutex.Unlock()

	if c == nil {
		return nil, errors.New("configuration must not be nil")
	} else if c.Cred == nil {
		return nil, errors.New("configuration must contain a valid Credential")
	}

	var name string
	if c.Name == "" {
		name = defaultName
	} else {
		name = c.Name
	}

	if _, exists := apps[name]; exists {
		var msg string
		if name == defaultName {
			msg = "The default Firebase app already exists. This means you called apps.New() multiple " +
				"times. If you want to initialize multiple apps, specify a unique name for each app " +
				"instance via the apps.Conf argument passed into apps.New()."
		} else {
			msg = fmt.Sprintf("Firebase app named %q already exists. This means you called apps.New() "+
				"multiple times with the same name argument. Make sure to provide a unique name in the "+
				"apps.Conf each time you call apps.New().", name)
		}
		return nil, errors.New(msg)
	}

	a := &appImpl{
		Conf:     &internal.AppConf{Name: name, Cred: c.Cred},
		Mutex:    &sync.Mutex{},
		Services: make(map[string]internal.AppService),
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
