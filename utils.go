package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

func encodeBody(obj interface{}) (io.Reader, error) {
	buffer := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buffer).Encode(obj); err != nil {
		return nil, err
	}
	return buffer, nil
}

func decodeBody(resp *http.Response, out interface{}) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}
