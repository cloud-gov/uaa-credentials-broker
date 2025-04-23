package main

import (
	"context"

	cfclient "github.com/cloudfoundry/go-cfclient/v3/client"
	cf "github.com/cloudfoundry/go-cfclient/v3/resource"
)

type PAASClient interface {
	ServiceInstanceByGuid(guid string) (*cf.ServiceInstance, error)
	GetSpaceByGuid(guid string) (*cf.Space, error)
	CreateUser(guid string) (*cf.User, error)
	DeleteUser(guid string) error
	AssociateOrgUserByUsername(orgID, userName string) (*cf.Role, error)
	AssociateOrgAuditorByUsername(orgID, userName string) (*cf.Role, error)
	AssociateSpaceDeveloperByUsername(spaceID, userName string) (*cf.Role, error)
	AssociateSpaceAuditorByUsername(spaceID, userName string) (*cf.Role, error)
}

type CFClient struct {
	Client *cfclient.Client
}

func (c *CFClient) ServiceInstanceByGuid(guid string) (*cf.ServiceInstance, error) {
	svcInst, err := c.Client.ServiceInstances.Get(context.Background(), guid)
	return svcInst, err
}

func (c *CFClient) GetSpaceByGuid(guid string) (*cf.Space, error) {
	space, err := c.Client.Spaces.Get(context.Background(), guid)
	return space, err
}

func (c *CFClient) GetOrganizationByGuid(guid string) (*cf.Organization, error) {
	org, err := c.Client.Organizations.Get(context.Background(), guid)
	return org, err
}

func (c *CFClient) CreateUser(guid string) (*cf.User, error) {
	user, err := c.Client.Users.Create(context.Background(), &cf.UserCreate{GUID: guid})
	return user, err
}

func (c *CFClient) DeleteUser(guid string) error {
	_, err := c.Client.Users.Delete(context.Background(), guid)
	return err
}

func (c *CFClient) AssociateOrgUserByUsernameAndRole(orgID, userName string, roleType cf.OrganizationRoleType) (*cf.Role, error) {
	role, err := c.Client.Roles.CreateOrganizationRoleWithUsername(context.Background(), orgID, userName, roleType, "")
	return role, err
}

func (c *CFClient) AssociateOrgUserByUsername(orgID, userName string) (*cf.Role, error) {
	return c.AssociateOrgUserByUsernameAndRole(orgID, userName, cf.OrganizationRoleUser)
}

func (c *CFClient) AssociateOrgAuditorByUsername(orgID, userName string) (*cf.Role, error) {
	return c.AssociateOrgUserByUsernameAndRole(orgID, userName, cf.OrganizationRoleAuditor)
}

func (c *CFClient) AssociateSpaceUserByUsernameAndRole(spaceID, userName string, roleType cf.SpaceRoleType) (*cf.Role, error) {
	role, err := c.Client.Roles.CreateSpaceRoleWithUsername(context.Background(), spaceID, userName, roleType, "")
	return role, err
}

func (c *CFClient) AssociateSpaceDeveloperByUsername(spaceID, userName string) (*cf.Role, error) {
	return c.AssociateSpaceUserByUsernameAndRole(spaceID, userName, cf.SpaceRoleDeveloper)
}

func (c *CFClient) AssociateSpaceAuditorByUsername(spaceID, userName string) (*cf.Role, error) {
	return c.AssociateSpaceUserByUsernameAndRole(spaceID, userName, cf.SpaceRoleAuditor)
}
