package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func CreateEphemeralLink(endpoint, body string, hours, maxViews int) (string, error) {
	data := map[string]interface{}{
		"message": map[string]interface{}{
			"body":      body,
			"hours":     hours,
			"max_views": maxViews,
		},
	}
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(data)

	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/m", endpoint), b)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 302 {
		return "", fmt.Errorf("Expected status 302; got %d", resp.StatusCode)
	}
	return resp.Header.Get("Location"), nil
}
