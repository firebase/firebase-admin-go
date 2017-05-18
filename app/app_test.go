package app

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/firebase/firebase-admin-go/internal"
)

var cred = &testCredential{}
var conf = &Conf{Cred: cred}

const googAppCreds string = "GOOGLE_APPLICATION_CREDENTIALS"

type testCredential struct{}

func (t *testCredential) AccessToken(ctx context.Context) (string, time.Time, error) {
	return "mock-token", time.Now().Add(time.Hour), nil
}

type testAppService struct {
	Val    string
	Delete bool
}

func (t *testAppService) Del() {
	t.Delete = true
}

func setGoogleAppCredentials(t *testing.T, path string) string {
	current := os.Getenv(googAppCreds)
	if err := os.Setenv(googAppCreds, path); err != nil {
		t.Fatal(err)
	}
	return current
}

func clearApps() {
	mutex.Lock()
	defer mutex.Unlock()
	for k := range apps {
		delete(apps, k)
	}
}

func TestNewApp(t *testing.T) {
	defer clearApps()

	got, err := New(&Conf{Cred: cred})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name() != defaultName {
		t.Errorf("Name: %q; want: %q", got.Name(), defaultName)
	}
	if got.Credential() != cred {
		t.Errorf("Credential: %v; want: %v", got.Credential(), cred)
	}

	if a, err := New(&Conf{Cred: cred}); err == nil {
		t.Errorf("New('default') = (%v, %v); want: (nil, error)", a, err)
	}
}

func TestNewAppWithName(t *testing.T) {
	defer clearApps()

	got, err := New(&Conf{Cred: cred, Name: "myApp"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name() != "myApp" {
		t.Errorf("Name: %q; want: %q", got.Name(), "myApp")
	}
	if got.Credential() != cred {
		t.Errorf("Credential: %v; want: %v", got.Credential(), cred)
	}

	if a, err := New(&Conf{Cred: cred, Name: "myApp"}); err == nil {
		t.Errorf("New('myApp') = (%v, %v); want: (nil, error)", a, err)
	}
}

func TestNewAppWithNoCredential(t *testing.T) {
	defer clearApps()

	got, err := New(&Conf{})
	if got != nil || err == nil {
		t.Errorf("New({}) = (%v, %v); want (nil, error)", got, err)
	}
}

func TestNewAppWithNil(t *testing.T) {
	defer clearApps()

	got, err := New(nil)
	if got != nil || err == nil {
		t.Errorf("New(nil) = (%v, %v); want (nil, error)", got, err)
	}
}

func TestNewAppWithDefaultsError(t *testing.T) {
	defer clearApps()

	current := setGoogleAppCredentials(t, "non_existing.json")
	defer setGoogleAppCredentials(t, current)

	if got, err := New(&Conf{}); err == nil {
		t.Errorf("New('default') = (%v, %v); want: (nil, error)", got, err)
	}
}

func TestMultipleNewApp(t *testing.T) {
	defer clearApps()

	if _, err := New(&Conf{Cred: cred}); err != nil {
		t.Fatal(err)
	}
	if _, err := New(&Conf{Cred: cred, Name: "myApp"}); err != nil {
		t.Fatal(err)
	}

	if got, err := New(&Conf{Cred: cred}); err == nil {
		t.Errorf("New('default') = (%v, %v); want: (nil, error)", got, err)
	}
	if got, err := New(&Conf{Cred: cred, Name: "myApp"}); err == nil {
		t.Errorf("New('myApp') = (%v, %v); want: (nil, error)", got, err)
	}
}

func TestGet(t *testing.T) {
	defer clearApps()

	app1, err := New(&Conf{Cred: cred})
	if err != nil {
		t.Fatal(err)
	}
	app2, err := New(&Conf{Cred: cred, Name: "myApp"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if got != app1 {
		t.Errorf("Default() = %v; want: %v", got, app1)
	}

	got, err = Get("myApp")
	if err != nil {
		t.Fatal(err)
	}
	if got != app2 {
		t.Errorf("Get('myApp') = %v; want: %v", got, app2)
	}
}

func TestGetNonExistingApp(t *testing.T) {
	got, err := Get("nonExisting")
	if got != nil || err == nil {
		t.Errorf("Get('nonExisting') = (%v, %v); want: (nil, error)", got, err)
	}
}

func TestDelete(t *testing.T) {
	defer clearApps()

	app1, err := New(&Conf{Cred: cred})
	if err != nil {
		t.Fatal(err)
	}
	app2, err := New(&Conf{Cred: cred, Name: "myApp"})
	if err != nil {
		t.Fatal(err)
	}

	app1.Del()
	app2.Del()

	if got, err := Default(); err == nil {
		t.Errorf("Default() = (%v, %v); want: (nil, error)", got, err)
	}
	if got, err := Get("myApp"); err == nil {
		t.Errorf("Get('myApp') = (%v, %v); want: (nil, error)", got, err)
	}
}

func TestMultipleDelete(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Del() did not panic; want panic")
		}
	}()

	a, err := New(&Conf{Cred: cred})
	if err != nil {
		t.Fatal(err)
	}

	a.Del()
	a.Del()
}

func TestReinitApp(t *testing.T) {
	defer clearApps()

	app1, err := New(&Conf{Cred: cred})
	if err != nil {
		t.Fatal(err)
	}
	app2, err := New(&Conf{Cred: cred, Name: "myApp"})
	if err != nil {
		t.Fatal(err)
	}

	app1.Del()
	app2.Del()

	app3, err := New(&Conf{Cred: cred})
	if err != nil {
		t.Fatal(err)
	}

	app4, err := New(&Conf{Cred: cred, Name: "myApp"})
	if err != nil {
		t.Fatal(err)
	}

	if app1 == app3 {
		t.Errorf("New('default') == New('default'); want not equal")
	}
	if app2 == app4 {
		t.Errorf("New('myApp') == New('myApp'); want not equal")
	}
}

func TestAppService(t *testing.T) {
	defer clearApps()

	a, err := New(&Conf{Cred: cred})
	if err != nil {
		t.Fatal(err)
	}

	var c int32
	fn := func() internal.AppService {
		c++
		return &testAppService{Val: "test"}
	}

	got := a.(*appImpl)
	s := got.service("test", fn).(*testAppService)
	if c != 1 {
		t.Errorf("Count: %d; want 1", c)
	}
	if s.Delete {
		t.Error("Delete: true; want: false")
	}

	if got.service("test", fn) == nil {
		t.Errorf("service() = nil; want value")
	}
	if c != 1 {
		t.Errorf("Count: %d; want 1", c)
	}

	got.Del()
	if !s.Delete {
		t.Error("Delete: false; want: true")
	}
}

func TestAuth(t *testing.T) {
	defer clearApps()

	app1, err := New(&Conf{Cred: cred})
	if err != nil {
		t.Fatal(err)
	}
	app2, err := New(&Conf{Cred: cred, Name: "myApp"})
	if err != nil {
		t.Fatal(err)
	}

	if s := app1.Auth(); s == nil {
		t.Error("Auth() = nil; want auth.Auth")
	}
	if s := app2.Auth(); s == nil {
		t.Error("Auth() = nil; want auth.Auth")
	}
}

func TestAuthAfterDeleteApp(t *testing.T) {
	defer clearApps()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Auth() did not panic; want panic")
		}
	}()

	app, err := New(&Conf{Cred: cred})
	if err != nil {
		t.Fatal(err)
	}
	app.Del()
	app.Auth()
}
