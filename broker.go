package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi"
)

type ProvisionOptions struct {
	RedirectURI []string `json:"redirect_uri"`
	Scopes      []string `json:"scopes"`
}

var (
	clientAccountGUID = "6b508bb8-2af7-4a75-9efd-7b76a01d705d"
	userAccountGUID   = "964bd86d-72fa-4852-957f-e4cd802de34b"
)

var (
	defaultScopes = []string{"openid"}
	allowedScopes = map[string]bool{
		"openid":                true,
		"cloud_controller.read": true,
	}
)

var catalog = []brokerapi.Service{
	{
		ID:          clientAccountGUID,
		Name:        "cloud-gov-identity-provider",
		Description: "Manage client credentials for authenticating cloud.gov users in your app",
		Plans: []brokerapi.ServicePlan{
			{
				ID:          "e6fd8aaa-b5ba-4b19-b52e-44c18ab8ca1d",
				Name:        "oauth-client",
				Description: "OAuth client credentials for authenticating cloud.gov users in your app",
			},
		},
	},
	{
		ID:          userAccountGUID,
		Name:        "cloud-gov-service-account",
		Description: "Manage cloud.gov service accounts with access to your organization",
		Plans: []brokerapi.ServicePlan{
			{
				ID:          "074e652b-b77b-4ac3-8d5b-52144486b1a3",
				Name:        "space-deployer",
				Description: "An account for continuous deployment, limited to a single space",
			},
		},
	},
}

type DeployerAccountBroker struct {
	uaaClient        AuthClient
	cfClient         PAASClient
	credentialSender CredentialSender
	generatePassword PasswordGenerator
	logger           lager.Logger
	config           Config
}

func (b *DeployerAccountBroker) Services(context context.Context) []brokerapi.Service {
	return catalog
}

func (b *DeployerAccountBroker) Provision(
	context context.Context,
	instanceID string,
	details brokerapi.ProvisionDetails,
	asyncAllowed bool,
) (brokerapi.ProvisionedServiceSpec, error) {
	b.logger.Info("provision", lager.Data{"instanceID": instanceID})

	var (
		link string
		err  error
	)

	password := b.generatePassword(b.config.PasswordLength)

	switch details.ServiceID {
	case clientAccountGUID:
		var opts ProvisionOptions
		if err := json.Unmarshal(details.RawParameters, &opts); err != nil {
			return brokerapi.ProvisionedServiceSpec{}, err
		}

		if len(opts.RedirectURI) == 0 {
			return brokerapi.ProvisionedServiceSpec{}, errors.New(`Must pass field "redirect_uri"`)
		}

		if _, err := b.provisionClient(instanceID, password, opts.RedirectURI, opts.Scopes); err != nil {
			return brokerapi.ProvisionedServiceSpec{}, err
		}

		link, err = b.credentialSender.Send(fmt.Sprintf("Client ID: %s\nClient Secret: %s", instanceID, password))
		if err != nil {
			return brokerapi.ProvisionedServiceSpec{}, err
		}
	case userAccountGUID:
		user, err := b.provisionUser(instanceID, password)
		if err != nil {
			return brokerapi.ProvisionedServiceSpec{}, err
		}
		err = b.cfClient.CreateUser(user.ID)
		if err != nil {
			return brokerapi.ProvisionedServiceSpec{}, err
		}

		err = b.cfClient.AddUserToOrg(user.ID, details.OrganizationGUID)
		if err != nil {
			return brokerapi.ProvisionedServiceSpec{}, err
		}
		err = b.cfClient.AddUserToSpace(user.ID, details.SpaceGUID)
		if err != nil {
			return brokerapi.ProvisionedServiceSpec{}, err
		}

		link, err = b.credentialSender.Send(fmt.Sprintf("Username: %s\nPassword: %s", instanceID, password))
		if err != nil {
			return brokerapi.ProvisionedServiceSpec{}, err
		}
	default:
		return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("Service ID %s not found", details.ServiceID)
	}

	return brokerapi.ProvisionedServiceSpec{
		IsAsync:      false,
		DashboardURL: link,
	}, nil
}

func (b *DeployerAccountBroker) provisionClient(clientID, clientSecret string, redirectURI []string, scopes []string) (Client, error) {
	if len(scopes) == 0 {
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

	return b.uaaClient.CreateClient(Client{
		ID:                   clientID,
		AuthorizedGrantTypes: []string{"authorization_code", "refresh_token"},
		Scope:                scopes,
		RedirectURI:          redirectURI,
		ClientSecret:         clientSecret,
		AccessTokenValidity:  b.config.AccessTokenValidity,
		RefreshTokenValidity: b.config.RefreshTokenValidity,
	})
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

func (b *DeployerAccountBroker) Deprovision(
	context context.Context,
	instanceID string,
	details brokerapi.DeprovisionDetails,
	asyncAllowed bool,
) (brokerapi.DeprovisionServiceSpec, error) {
	switch details.ServiceID {
	case clientAccountGUID:
		if err := b.uaaClient.DeleteClient(instanceID); err != nil {
			return brokerapi.DeprovisionServiceSpec{}, err
		}
	case userAccountGUID:
		user, err := b.uaaClient.GetUser(instanceID)
		if err != nil {
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
