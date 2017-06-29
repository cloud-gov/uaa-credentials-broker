package main

import (
	"github.com/cloudfoundry-community/go-cfclient"
)

type PAASClient interface {
	ServiceInstanceByGuid(guid string) (cfclient.ServiceInstance, error)
	GetSpaceByGuid(guid string) (cfclient.Space, error)
	CreateUser(req cfclient.UserRequest) (cfclient.User, error)
	DeleteUser(userID string) error
	AssociateOrgUserByUsername(orgID, userName string) (cfclient.Org, error)
	AssociateOrgAuditorByUsername(orgID, userName string) (cfclient.Org, error)
	AssociateSpaceDeveloperByUsername(spaceID, userName string) (cfclient.Space, error)
	AssociateSpaceAuditorByUsername(spaceID, userName string) (cfclient.Space, error)
}
