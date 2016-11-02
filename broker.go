package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi"
)

type Users struct {
	Resources    []User
	TotalResults int
}

type User struct {
	ID       string  `json:"id,omitempty"`
	UserName string  `json:"userName,omitempty"`
	Password string  `json:"password,omitempty"`
	Active   bool    `json:"active,omitempty"`
	Emails   []Email `json:"emails"`
}

type Email struct {
	Value   string `json:"value,omitempty"`
	Primary bool   `json:"primary"`
}

type DeployerAccountBroker struct {
	uaaClient        *http.Client
	cfClient         *http.Client
	credentialSender CredentialSender
	logger           lager.Logger
	config           Config
}

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

func (b *DeployerAccountBroker) Services(context context.Context) []brokerapi.Service {
	return []brokerapi.Service{{
		ID:          "964bd86d-72fa-4852-957f-e4cd802de34b",
		Name:        "deployer-account",
		Description: "Deployer account",
		Plans: []brokerapi.ServicePlan{{
			ID:          "074e652b-b77b-4ac3-8d5b-52144486b1a3",
			Name:        "deployer-account",
			Description: "Deployer account",
		}},
	}}
}

func (b *DeployerAccountBroker) Provision(
	context context.Context,
	instanceID string,
	details brokerapi.ProvisionDetails,
	asyncAllowed bool,
) (brokerapi.ProvisionedServiceSpec, error) {
	b.logger.Info("provision", lager.Data{"instanceID": instanceID})

	password := GenerateSecurePassword(b.config.PasswordLength)

	user, err := b.provisionUser(instanceID, password)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, err
	}
	err = b.provisionCFUser(user.ID)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	err = b.addUserToOrg(details.OrganizationGUID, user.ID)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, err
	}
	err = b.addUserToSpace(details.SpaceGUID, user.ID)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	link, err := b.credentialSender.Send(fmt.Sprintf("%s | %s", instanceID, password))
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	return brokerapi.ProvisionedServiceSpec{
		IsAsync:      false,
		DashboardURL: link,
	}, nil
}

func (b *DeployerAccountBroker) provisionUser(userID, password string) (User, error) {
	b.logger.Info("create-uaa-user", lager.Data{"userID": userID})

	user := User{
		UserName: userID,
		Password: password,
		Emails: []Email{{
			Value:   b.config.EmailAddress,
			Primary: true,
		}},
	}

	body, _ := encodeBody(user)
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/Users", b.config.UAAAddress), body)
	req.Header.Add("X-Identity-Zone-Id", b.config.UAAZone)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	resp, err := b.uaaClient.Do(req)
	if err != nil {
		return User{}, err
	}

	err = decodeBody(resp, &user)
	if err != nil {
		return User{}, err
	}

	return user, nil
}

func (b *DeployerAccountBroker) provisionCFUser(userID string) error {
	b.logger.Info("create-cf-user", lager.Data{"userID": userID})

	body, _ := encodeBody(map[string]string{"guid": userID})
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v2/users", b.config.CFAddress), body)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	resp, err := b.cfClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		return fmt.Errorf("Expected status 201; got: %d", resp.StatusCode)
	}

	return nil
}

func (b *DeployerAccountBroker) getUser(userID string) (User, error) {
	b.logger.Info("get-user", lager.Data{"user": userID})

	u, _ := url.Parse(fmt.Sprintf("%s/Users", b.config.UAAAddress))
	q := u.Query()
	q.Add("filter", fmt.Sprintf(`userName eq "%s"`, userID))
	q.Add("count", "1")
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Add("X-Identity-Zone-Id", b.config.UAAZone)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	resp, err := b.uaaClient.Do(req)
	if err != nil {
		return User{}, err
	}

	users := Users{}
	err = decodeBody(resp, &users)
	if err != nil {
		return User{}, err
	}

	if users.TotalResults != 1 {
		return User{}, fmt.Errorf("Expected to find exactly one user; got %d", users.TotalResults)
	}

	return users.Resources[0], nil
}

func (b *DeployerAccountBroker) deprovisionUser(userID string) error {
	b.logger.Info("delete-uaa-user", lager.Data{"userID": userID})

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/Users/%s", b.config.UAAAddress, userID), nil)
	req.Header.Add("X-Identity-Zone-Id", b.config.UAAZone)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	resp, err := b.uaaClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Expected status 200; got: %d", resp.StatusCode)
	}

	return nil
}

func (b *DeployerAccountBroker) deprovisionCFUser(userID string) error {
	b.logger.Info("delete-cf-user", lager.Data{"user": "user"})

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/v2/users/%s", b.config.CFAddress, userID), nil)
	resp, err := b.cfClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 204 {
		return fmt.Errorf("Expected status 204; got: %d", resp.StatusCode)
	}

	return nil
}

func (b *DeployerAccountBroker) addUserToOrg(orgID, userID string) error {
	b.logger.Info("add-org-user", lager.Data{"id": userID})

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/v2/organizations/%s/users/%s", b.config.CFAddress, orgID, userID), nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	resp, err := b.cfClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		return fmt.Errorf("Expected status 201; got: %d", resp.StatusCode)
	}

	return nil
}

func (b *DeployerAccountBroker) addUserToSpace(spaceID, userID string) error {
	b.logger.Info("add-space-developer", lager.Data{"id": userID})

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/v2/spaces/%s/developers/%s", b.config.CFAddress, spaceID, userID), nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	resp, err := b.cfClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		return fmt.Errorf("Expected status 201; got: %d", resp.StatusCode)
	}

	return nil
}

func (b *DeployerAccountBroker) Deprovision(
	context context.Context,
	instanceID string,
	details brokerapi.DeprovisionDetails,
	asyncAllowed bool,
) (brokerapi.DeprovisionServiceSpec, error) {
	user, err := b.getUser(instanceID)
	if err != nil {
		return brokerapi.DeprovisionServiceSpec{}, err
	}

	err = b.deprovisionCFUser(user.ID)
	if err != nil {
		return brokerapi.DeprovisionServiceSpec{}, err
	}

	err = b.deprovisionUser(user.ID)
	if err != nil {
		return brokerapi.DeprovisionServiceSpec{}, err
	}

	return brokerapi.DeprovisionServiceSpec{IsAsync: false}, nil
}

func (b *DeployerAccountBroker) Bind(context context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	return brokerapi.Binding{}, errors.New("Broker does not support bind")
}

func (b *DeployerAccountBroker) Unbind(context context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails) error {
	return nil
}

func (b *DeployerAccountBroker) Update(context context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	return brokerapi.UpdateServiceSpec{}, errors.New("Broker does not support update")
}

func (b *DeployerAccountBroker) LastOperation(context context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, errors.New("Broker does not support last operation")
}
