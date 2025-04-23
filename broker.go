package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi"

	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type BindOptions struct {
	RedirectURI []string `json:"redirect_uri"`
	Scopes      []string `json:"scopes"`
	AllowPublic *bool    `json:"allowpublic"`
}

var (
	clientAccountGUID = "6b508bb8-2af7-4a75-9efd-7b76a01d705d"
	userAccountGUID   = "964bd86d-72fa-4852-957f-e4cd802de34b"
	deployerGUID      = "074e652b-b77b-4ac3-8d5b-52144486b1a3"
	auditorGUID       = "dc3a6d48-9622-434a-b418-1d920193b575"
)

var (
	defaultScopes = []string{"openid"}
	allowedScopes = map[string]bool{
		"openid": true,
	}
)

type DeployerAccountBroker struct {
	uaaClient        AuthClient
	cfClient         PAASClient
	generatePassword PasswordGenerator
	logger           lager.Logger
	config           Config
}

func (b *DeployerAccountBroker) Services(context context.Context) []brokerapi.Service {
	var services []brokerapi.Service
	pwd, _ := os.Getwd()

	buf, err := ioutil.ReadFile(filepath.Join(pwd, "config.json"))
	if err != nil {
		b.logger.Error("services", err)
		return []brokerapi.Service{}
	}
	err = json.Unmarshal(buf, &services)
	if err != nil {
		return []brokerapi.Service{}
	}
	return services
}

func (b *DeployerAccountBroker) Provision(
	context context.Context,
	instanceID string,
	details brokerapi.ProvisionDetails,
	asyncAllowed bool,
) (brokerapi.ProvisionedServiceSpec, error) {
	return brokerapi.ProvisionedServiceSpec{}, nil
}

func (b *DeployerAccountBroker) Deprovision(
	context context.Context,
	instanceID string,
	details brokerapi.DeprovisionDetails,
	asyncAllowed bool,
) (brokerapi.DeprovisionServiceSpec, error) {
	// Handle instances created before credential management was moved to bind and unbind
	switch details.ServiceID {
	case clientAccountGUID:
		if err := b.deleteClient(instanceID); err != nil {
			return brokerapi.DeprovisionServiceSpec{}, err
		}
	case userAccountGUID:
		user, err := b.uaaClient.GetUser(instanceID)
		if err != nil {
			if strings.Contains(err.Error(), "got 0") {
				return brokerapi.DeprovisionServiceSpec{}, nil
			}
			return brokerapi.DeprovisionServiceSpec{}, err
		}

		err = b.cfClient.DeleteUser(user.ID)
		if err != nil {
			return brokerapi.DeprovisionServiceSpec{}, err
		}

		err = b.uaaClient.DeleteUser(user.ID)
		if err != nil {
			return brokerapi.DeprovisionServiceSpec{}, err
		}
	default:
		return brokerapi.DeprovisionServiceSpec{}, fmt.Errorf("Service ID %s not found", details.ServiceID)
	}

	return brokerapi.DeprovisionServiceSpec{}, nil
}

func parseBindOptions(details brokerapi.BindDetails) (BindOptions, error) {
	opts := BindOptions{}

	if len(details.RawParameters) == 0 {
		return opts, errors.New(`must pass JSON configuration with field "redirect_uri"`)
	}

	if err := json.Unmarshal(details.RawParameters, &opts); err != nil {
		return opts, err
	}

	if len(opts.RedirectURI) == 0 {
		return opts, errors.New(`must pass field "redirect_uri"`)
	}

	return opts, nil
}

func (b *DeployerAccountBroker) Bind(
	context context.Context,
	instanceID, bindingID string,
	details brokerapi.BindDetails,
) (brokerapi.Binding, error) {
	password := b.generatePassword(b.config.PasswordLength)

	switch details.ServiceID {
	case clientAccountGUID:
		opts, err := parseBindOptions(details)
		if err != nil {
			return brokerapi.Binding{}, err
		}

		if _, err := b.provisionClient(bindingID, password, opts); err != nil {
			return brokerapi.Binding{}, err
		}

		return brokerapi.Binding{
			Credentials: map[string]string{
				"client_id":     bindingID,
				"client_secret": password,
			},
		}, nil
	case userAccountGUID:
		instance, err := b.cfClient.ServiceInstanceByGuid(instanceID)
		if err != nil {
			return brokerapi.Binding{}, err
		}

		space, err := b.cfClient.GetSpaceByGuid(instance.Relationships.Space.Data.GUID)
		if err != nil {
			return brokerapi.Binding{}, err
		}

		user, err := b.provisionUser(bindingID, password)
		if err != nil {
			return brokerapi.Binding{}, err
		}
		_, err = b.cfClient.CreateUser(user.ID)
		if err != nil {
			return brokerapi.Binding{}, err
		}

		_, err = b.cfClient.AssociateOrgUserByUsername(space.Relationships.Organization.Data.GUID, user.UserName)
		if err != nil {
			return brokerapi.Binding{}, err
		}

		switch details.PlanID {
		case deployerGUID:
			_, err = b.cfClient.AssociateSpaceDeveloperByUsername(instance.Relationships.Space.Data.GUID, user.UserName)
			if err != nil {
				return brokerapi.Binding{}, err
			}
		case auditorGUID:
			_, err = b.cfClient.AssociateSpaceAuditorByUsername(instance.Relationships.Space.Data.GUID, user.UserName)
			if err != nil {
				return brokerapi.Binding{}, err
			}
		}

		return brokerapi.Binding{
			Credentials: map[string]string{
				"username": bindingID,
				"password": password,
			},
		}, nil
	default:
		return brokerapi.Binding{}, fmt.Errorf("Service ID %s not found", details.ServiceID)
	}

	return brokerapi.Binding{}, nil
}

func (b *DeployerAccountBroker) Unbind(
	context context.Context,
	instanceID,
	bindingID string,
	details brokerapi.UnbindDetails,
) error {
	switch details.ServiceID {
	case clientAccountGUID:
		err := b.deleteClient(bindingID)
		if err != nil {
			return err
		}
	case userAccountGUID:
		user, err := b.uaaClient.GetUser(bindingID)
		if err != nil {
			if strings.Contains(err.Error(), "got 0") {
				return nil
			}
			return err
		}

		err = b.cfClient.DeleteUser(user.ID)
		if err != nil {
			return err
		}

		err = b.uaaClient.DeleteUser(user.ID)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Service ID %s not found", details.ServiceID)
	}

	return nil
}

func (b *DeployerAccountBroker) Update(context context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	return brokerapi.UpdateServiceSpec{}, errors.New("Broker does not support update")
}

func (b *DeployerAccountBroker) LastOperation(context context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, errors.New("Broker does not support last operation")
}

func (b *DeployerAccountBroker) provisionClient(
	clientID,
	clientSecret string,
	opts BindOptions,
) (Client, error) {
	var scopes = opts.Scopes
	if len(opts.Scopes) == 0 {
		scopes = defaultScopes
	}
	forbiddenScopes := []string{}
	for _, scope := range scopes {
		if _, ok := allowedScopes[scope]; !ok {
			forbiddenScopes = append(forbiddenScopes, scope)
		}
	}
	if len(forbiddenScopes) > 0 {
		return Client{}, fmt.Errorf("Scope(s) not permitted: %s", strings.Join(forbiddenScopes, ", "))
	}

	client := Client{
		ID:                   clientID,
		AuthorizedGrantTypes: []string{"authorization_code", "refresh_token"},
		Scope:                scopes,
		RedirectURI:          opts.RedirectURI,
		ClientSecret:         clientSecret,
		AccessTokenValidity:  b.config.AccessTokenValidity,
		RefreshTokenValidity: b.config.RefreshTokenValidity,
	}

	if opts.AllowPublic != nil {
		client.AllowPublic = *opts.AllowPublic
	}

	return b.uaaClient.CreateClient(client)
}

func (b *DeployerAccountBroker) deleteClient(
	clientID string,
) error {
	err := b.uaaClient.DeleteClient(clientID)

	// Allow 404 responses on deletion
	if err != nil && strings.Contains(err.Error(), "404") {
		return nil
	}

	return err
}

func (b *DeployerAccountBroker) provisionUser(userID, password string) (User, error) {
	user := User{
		UserName: userID,
		Password: password,
		Emails: []Email{{
			Value:   b.config.EmailAddress,
			Primary: true,
		}},
	}

	return b.uaaClient.CreateUser(user)
}
