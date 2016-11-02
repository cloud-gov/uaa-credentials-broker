package main

import (
	"fmt"
	"net/http"
)

type CredentialSender interface {
	Send(message string) (string, error)
}

type FugaciousCredentialSender struct {
	endpoint string
	hours    int
	maxViews int
}

func (f FugaciousCredentialSender) Send(message string) (string, error) {
	data := map[string]interface{}{
		"message": map[string]interface{}{
			"body":      message,
			"hours":     f.hours,
			"max_views": f.maxViews,
		},
	}

	body, _ := encodeBody(data)
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/m", f.endpoint), body)
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
	if resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("Expected status %d; got %d", http.StatusFound, resp.StatusCode)
	}
	return resp.Header.Get("Location"), nil
}
