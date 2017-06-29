package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/pivotal-cf/brokerapi"
)

type BindOptions struct {
	RedirectURI []string `json:"redirect_uri"`
	Scopes      []string `json:"scopes"`
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
		"openid":                true,
		"cloud_controller.read": true,
	}
)

var catalog = []brokerapi.Service{
	{
		ID:          clientAccountGUID,
		Name:        "cloud-gov-identity-provider",
		Description: "Manage client credentials for authenticating cloud.gov users in your app",
		Bindable:    true,
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
		Bindable:    true,
		Plans: []brokerapi.ServicePlan{
			{
				ID:          deployerGUID,
				Name:        "space-deployer",
				Description: "A service account for continuous deployment, limited to a single space",
			},
			{
				ID:          auditorGUID,
				Name:        "space-auditor",
				Description: "A service account for auditing configuration and monitoring events limited to a single space",
			},
		},
	},
}

type DeployerAccountBroker struct {
	uaaClient        AuthClient
	cfClient         PAASClient
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
	return brokerapi.ProvisionedServiceSpec{}, nil
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
	return brokerapi.DeprovisionServiceSpec{}, nil
}

func (b *DeployerAccountBroker) Bind(
	context context.Context,
	instanceID, bindingID string,
	details brokerapi.BindDetails,
) (brokerapi.Binding, error) {
	password := b.generatePassword(b.config.PasswordLength)

	switch details.ServiceID {
	case clientAccountGUID:
		var opts BindOptions
		if err := json.Unmarshal(details.RawParameters, &opts); err != nil {
			return brokerapi.Binding{}, err
		}

		if len(opts.RedirectURI) == 0 {
			return brokerapi.Binding{}, errors.New(`Must pass field "redirect_uri"`)
		}

		if _, err := b.provisionClient(bindingID, password, opts.RedirectURI, opts.Scopes); err != nil {
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

		space, err := b.cfClient.GetSpaceByGuid(instance.SpaceGuid)
		if err != nil {
			return brokerapi.Binding{}, err
		}

		user, err := b.provisionUser(bindingID, password)
		if err != nil {
			return brokerapi.Binding{}, err
		}
		_, err = b.cfClient.CreateUser(cfclient.UserRequest{Guid: user.ID})
		if err != nil {
			return brokerapi.Binding{}, err
		}

		_, err = b.cfClient.AssociateOrgUserByUsername(space.OrganizationGuid, user.UserName)
		if err != nil {
			return brokerapi.Binding{}, err
		}

		switch details.PlanID {
		case deployerGUID:
			_, err = b.cfClient.AssociateSpaceDeveloperByUsername(instance.SpaceGuid, user.UserName)
			if err != nil {
				return brokerapi.Binding{}, err
			}
		case auditorGUID:
			_, err = b.cfClient.AssociateSpaceAuditorByUsername(instance.SpaceGuid, user.UserName)
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
	}

	return brokerapi.Binding{}, fmt.Errorf("Service ID %s not found", details.ServiceID)
}

func (b *DeployerAccountBroker) Unbind(
	context context.Context,
	instanceID, bindingID string,
	details brokerapi.UnbindDetails,
) error {
	switch details.ServiceID {
	case clientAccountGUID:
		if err := b.uaaClient.DeleteClient(bindingID); err != nil {
			return err
		}
	case userAccountGUID:
		user, err := b.uaaClient.GetUser(bindingID)
		if err != nil {
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
