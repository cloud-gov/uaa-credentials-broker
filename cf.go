package main

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
)

type PAASClient interface {
	CreateUser(userID string) error
	DeleteUser(userID string) error
	AddUserToOrg(userID, orgID string) error
	AddUserToSpace(userID, spaceID string) error
}

type CFClient struct {
	logger   lager.Logger
	client   *http.Client
	endpoint string
}

func (c *CFClient) CreateUser(userID string) error {
	c.logger.Info("cf-create-user", lager.Data{"userID": userID})

	body, _ := encodeBody(map[string]string{"guid": userID})
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v2/users", c.endpoint), body)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		return fmt.Errorf("Expected status 201; got: %d", resp.StatusCode)
	}

	return nil
}

func (c *CFClient) DeleteUser(userID string) error {
	c.logger.Info("cf-delete-user", lager.Data{"userID": userID})

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/v2/users/%s", c.endpoint, userID), nil)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 204 {
		return fmt.Errorf("Expected status 204; got: %d", resp.StatusCode)
	}

	return nil
}

func (c *CFClient) AddUserToOrg(userID, orgID string) error {
	c.logger.Info("cf-add-org-user", lager.Data{"userID": userID, "orgID": orgID})

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/v2/organizations/%s/users/%s", c.endpoint, orgID, userID), nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		return fmt.Errorf("Expected status 201; got: %d", resp.StatusCode)
	}

	return nil
}

func (c *CFClient) AddUserToSpace(userID, spaceID string) error {
	c.logger.Info("cf-add-org-user", lager.Data{"userID": userID, "spaceID": spaceID})

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/v2/spaces/%s/developers/%s", c.endpoint, spaceID, userID), nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		return fmt.Errorf("Expected status 201; got: %d", resp.StatusCode)
	}

	return nil
}
