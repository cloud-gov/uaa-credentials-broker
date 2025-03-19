package main

import (
	"bytes"
	"encoding/json"
	"io"
)

func encodeBody(obj interface{}) (io.Reader, error) {
	buffer := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buffer).Encode(obj); err != nil {
		return nil, err
	}
	return buffer, nil
}

func decodeBody(body io.ReadCloser, out interface{}) error {
	defer body.Close()
	return json.NewDecoder(body).Decode(out)
}
