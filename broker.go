package main

import (
	"context"
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi"
)

type DeployerAccountBroker struct {
	uaaClient        AuthClient
	cfClient         PAASClient
	credentialSender CredentialSender
	generatePassword PasswordGenerator
	logger           lager.Logger
	config           Config
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

	password := b.generatePassword(b.config.PasswordLength)

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

	link, err := b.credentialSender.Send(fmt.Sprintf("Username: %s \n Password: %s", instanceID, password))
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	return brokerapi.ProvisionedServiceSpec{
		IsAsync:      false,
		DashboardURL: link,
	}, nil
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
