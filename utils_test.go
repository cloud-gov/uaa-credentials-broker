package main

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeBody(t *testing.T) {
	body := `{"foo":"bar"}`
	output := map[string]any{}
	req, err := http.NewRequest("post", "fake-url", strings.NewReader(body))
	if err != nil {
		t.Error(err)
	}

	err = decodeBody(req.Body, &output)
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(output, map[string]any{"foo": "bar"}) {
		t.Error("output does not equal expected value")
	}
}
