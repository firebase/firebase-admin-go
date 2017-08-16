package integration

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/firebase/firebase-admin-go/auth"
	"github.com/firebase/firebase-admin-go/integration/internal"

	"golang.org/x/net/context"
)

const idToolKitURL = "https://www.googleapis.com/identitytoolkit/v3/relyingparty/verifyCustomToken?key=%s"

var client *auth.Client

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		fmt.Println("skipping auth integration tests in short mode.")
		os.Exit(0)
	}

	app, err := internal.NewTestApp(context.Background())
	if err != nil {
		os.Exit(1)
	}

	client, err = app.Auth()
	if err != nil {
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestCustomToken(t *testing.T) {
	ct, err := client.CustomToken("user1")
	if err != nil {
		t.Fatal(err)
	}

	idt, err := signInWithCustomToken(ct)
	if err != nil {
		t.Fatal(err)
	}

	vt, err := client.VerifyIDToken(idt)
	if err != nil {
		t.Fatal(err)
	}
	if vt.UID != "user1" {
		t.Errorf("UID = %q; want UID = %q", vt.UID, "user1")
	}
}

func TestCustomTokenWithClaims(t *testing.T) {
	ct, err := client.CustomTokenWithClaims("user1", map[string]interface{}{
		"premium": true,
		"package": "gold",
	})
	if err != nil {
		t.Fatal(err)
	}

	idt, err := signInWithCustomToken(ct)
	if err != nil {
		t.Fatal(err)
	}

	vt, err := client.VerifyIDToken(idt)
	if err != nil {
		t.Fatal(err)
	}
	if vt.UID != "user1" {
		t.Errorf("UID = %q; want UID = %q", vt.UID, "user1")
	}
	if premium, ok := vt.Claims["premium"].(bool); !ok || !premium {
		t.Errorf("Claims['premium'] = %v; want Claims['premium'] = true", vt.Claims["premium"])
	}
	if pkg, ok := vt.Claims["package"].(string); !ok || pkg != "gold" {
		t.Errorf("Claims['package'] = %v; want Claims['package'] = \"gold\"", vt.Claims["package"])
	}
}

func signInWithCustomToken(token string) (string, error) {
	req, err := json.Marshal(map[string]interface{}{
		"token":             token,
		"returnSecureToken": true,
	})
	if err != nil {
		return "", err
	}

	apiKey, err := internal.APIKey()
	if err != nil {
		return "", err
	}

	resp, err := postRequest(fmt.Sprintf(idToolKitURL, apiKey), req)
	if err != nil {
		return "", err
	}
	var respBody struct {
		IDToken string `json:"idToken"`
	}
	if err := json.Unmarshal(resp, &respBody); err != nil {
		return "", err
	}
	return respBody.IDToken, err
}

func postRequest(url string, req []byte) ([]byte, error) {
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(req))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
