package db

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"

	"io/ioutil"

	"encoding/json"

	"reflect"

	"firebase.google.com/go/db"
	"firebase.google.com/go/integration/internal"
)

var client *db.Client
var ref *db.Ref
var testData map[string]interface{}
var parsedTestData map[string]Dinosaur

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("skipping database integration tests in short mode.")
		os.Exit(0)
	}

	ctx := context.Background()
	app, err := internal.NewTestApp(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	client, err = app.Database(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	ref, err = client.NewRef("_adminsdk/go/dinodb")
	if err != nil {
		log.Fatalln(err)
	}
	setup()

	os.Exit(m.Run())
}

func setup() {
	b, err := ioutil.ReadFile(internal.Resource("dinosaurs.json"))
	if err != nil {
		log.Fatalln(err)
	}
	if err = json.Unmarshal(b, &testData); err != nil {
		log.Fatalln(err)
	}

	b, err = json.Marshal(testData["dinosaurs"])
	if err != nil {
		log.Fatalln(err)
	}
	if err = json.Unmarshal(b, &parsedTestData); err != nil {
		log.Fatalln(err)
	}

	if err = ref.Set(testData); err != nil {
		log.Fatalln(err)
	}
}

func TestRef(t *testing.T) {
	if ref.Key != "dinodb" {
		t.Errorf("Key = %q; want = %q", ref.Key, "dinodb")
	}
	if ref.Path != "/_adminsdk/go/dinodb" {
		t.Errorf("Path = %q; want = %q", ref.Path, "/_adminsdk/go/dinodb")
	}
}

func TestChild(t *testing.T) {
	c, err := ref.Child("dinosaurs")
	if err != nil {
		t.Fatal(err)
	}
	if c.Key != "dinosaurs" {
		t.Errorf("Key = %q; want = %q", c.Key, "dinosaurs")
	}
	if c.Path != "/_adminsdk/go/dinodb/dinosaurs" {
		t.Errorf("Path = %q; want = %q", c.Path, "/_adminsdk/go/dinodb/dinosaurs")
	}
}

func TestParent(t *testing.T) {
	p := ref.Parent()
	if p.Key != "go" {
		t.Errorf("Key = %q; want = %q", p.Key, "go")
	}
	if p.Path != "/_adminsdk/go" {
		t.Errorf("Path = %q; want = %q", p.Path, "/_adminsdk/go")
	}
}

func TestGet(t *testing.T) {
	var m map[string]interface{}
	if err := ref.Get(&m); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testData, m) {
		t.Errorf("Get() = %v; want = %v", m, testData)
	}
}

func TestGetWithETag(t *testing.T) {
	var m map[string]interface{}
	etag, err := ref.GetWithETag(&m)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testData, m) {
		t.Errorf("GetWithETag() = %v; want = %v", m, testData)
	}
	if etag == "" {
		t.Errorf("GetWithETag() = \"\"; want non-empty")
	}
}

func TestGetIfChanged(t *testing.T) {
	var m map[string]interface{}
	ok, etag, err := ref.GetIfChanged("wrong-etag", &m)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || etag == "" {
		t.Errorf("GetIfChanged() = (%v, %q); want = (%v, %q)", ok, etag, true, "non-empty")
	}
	if !reflect.DeepEqual(testData, m) {
		t.Errorf("GetWithETag() = %v; want = %v", m, testData)
	}

	var m2 map[string]interface{}
	ok, etag2, err := ref.GetIfChanged(etag, &m2)
	if err != nil {
		t.Fatal(err)
	}
	if ok || etag != etag2 {
		t.Errorf("GetIfChanged() = (%v, %q); want = (%v, %q)", ok, etag2, false, etag)
	}
	if len(m2) != 0 {
		t.Errorf("GetWithETag() = %v; want empty", m)
	}
}

func TestGetChildValue(t *testing.T) {
	c, err := ref.Child("dinosaurs")
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	if err := c.Get(&m); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testData["dinosaurs"], m) {
		t.Errorf("Get() = %v; want = %v", m, testData["dinosaurs"])
	}
}

func TestGetGrandChildValue(t *testing.T) {
	c, err := ref.Child("dinosaurs/lambeosaurus")
	if err != nil {
		t.Fatal(err)
	}

	var got Dinosaur
	if err := c.Get(&got); err != nil {
		t.Fatal(err)
	}
	want := parsedTestData["lambeosaurus"]
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestGetNonExistingChild(t *testing.T) {
	c, err := ref.Child("non_existing")
	if err != nil {
		t.Fatal(err)
	}

	var i interface{}
	if err := c.Get(&i); err != nil {
		t.Fatal(err)
	}
	if i != nil {
		t.Errorf("Get() = %v; want nil", i)
	}
}

type Dinosaur struct {
	Appeared int     `json:"appeared"`
	Height   float64 `json:"height"`
	Length   float64 `json:"length"`
	Order    string  `json:"order"`
	Vanished int     `json:"vanished"`
	Weight   int     `json:"weight"`
	Ratings  Ratings `json:"ratings"`
}

type Ratings struct {
	Pos int `json:"pos"`
}
