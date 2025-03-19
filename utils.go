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

func decodeBody(bodyStream io.ReadCloser, out interface{}) error {
	defer bodyStream.Close()
	return json.NewDecoder(bodyStream).Decode(out)
}
