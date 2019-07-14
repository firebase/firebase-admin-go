// Copyright 2019 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package messaging

import "testing"

func TestMultipartEntitySingle(t *testing.T) {
	entity := &multipartEntity{
		parts: []*part{
			{
				method: "POST",
				url:    "http://example.com",
				body:   map[string]interface{}{"key": "value"},
			},
		},
	}

	const wantMime = "multipart/mixed; boundary=__END_OF_PART__"
	mime := entity.Mime()
	if mime != wantMime {
		t.Errorf("Mime() = %q; want = %q", mime, wantMime)
	}

	b, err := entity.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	want := "--__END_OF_PART__\r\n" +
		"Content-Id: 1\r\n" +
		"Content-Length: 118\r\n" +
		"Content-Transfer-Encoding: binary\r\n" +
		"Content-Type: application/http\r\n" +
		"\r\n" +
		"POST http://example.com HTTP/1.1\r\n" +
		"Content-Length: 15\r\n" +
		"Content-Type: application/json; charset=UTF-8\r\n" +
		"\r\n" +
		"{\"key\":\"value\"}\r\n" +
		"--__END_OF_PART__--\r\n"
	if string(b) != want {
		t.Errorf("Bytes() = %q; want = %q", string(b), want)
	}
}

func TestMultipartEntity(t *testing.T) {
	entity := &multipartEntity{
		parts: []*part{
			{
				method: "POST",
				url:    "http://example1.com",
				body:   map[string]interface{}{"key1": "value"},
			},
			{
				method:  "POST",
				url:     "http://example2.com",
				body:    map[string]interface{}{"key2": "value"},
				headers: map[string]string{"Custom-Header": "custom-value"},
			},
		},
	}

	const wantMime = "multipart/mixed; boundary=__END_OF_PART__"
	mime := entity.Mime()
	if mime != wantMime {
		t.Errorf("Mime() = %q; want = %q", mime, wantMime)
	}

	b, err := entity.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	want := "--__END_OF_PART__\r\n" +
		"Content-Id: 1\r\n" +
		"Content-Length: 120\r\n" +
		"Content-Transfer-Encoding: binary\r\n" +
		"Content-Type: application/http\r\n" +
		"\r\n" +
		"POST http://example1.com HTTP/1.1\r\n" +
		"Content-Length: 16\r\n" +
		"Content-Type: application/json; charset=UTF-8\r\n" +
		"\r\n" +
		"{\"key1\":\"value\"}\r\n" +
		"--__END_OF_PART__\r\n" +
		"Content-Id: 2\r\n" +
		"Content-Length: 149\r\n" +
		"Content-Transfer-Encoding: binary\r\n" +
		"Content-Type: application/http\r\n" +
		"\r\n" +
		"POST http://example2.com HTTP/1.1\r\n" +
		"Content-Length: 16\r\n" +
		"Content-Type: application/json; charset=UTF-8\r\n" +
		"Custom-Header: custom-value\r\n" +
		"\r\n" +
		"{\"key2\":\"value\"}\r\n" +
		"--__END_OF_PART__--\r\n"
	if string(b) != want {
		t.Errorf("multipartPayload() = %q; want = %q", string(b), want)
	}
}
