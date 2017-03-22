package main

import (
	"context"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/pivotal-cf/brokerapi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/18F/uaa-credentials-broker/mocks"
)

type FakeUAAClient struct {
	mock.Mock
	userGUID   string
	userName   string
	clientGUID string
}

func (c *FakeUAAClient) CreateClient(client Client) (Client, error) {
	c.Called(client)
	return Client{ID: c.clientGUID}, nil
}

func (c *FakeUAAClient) DeleteClient(clientID string) error {
	c.Called(clientID)
	return nil
}

func (c *FakeUAAClient) GetUser(userID string) (User, error) {
	c.Called(userID)
	return User{ID: c.userGUID}, nil
}

func (c *FakeUAAClient) CreateUser(user User) (User, error) {
	c.Called(user)
	return User{ID: c.userGUID, UserName: c.userName}, nil
}

func (c *FakeUAAClient) DeleteUser(userID string) error {
	c.Called(userID)
	return nil
}

type FakeCredentialSender struct {
	mock.Mock
	link string
}

func (s FakeCredentialSender) Send(message string) (string, error) {
	s.Called(message)
	return s.link, nil
}

var _ = Describe("broker", func() {
	var (
		uaaClient        FakeUAAClient
		cfClient         mocks.PAASClient
		credentialSender FakeCredentialSender
		broker           DeployerAccountBroker
	)

	BeforeEach(func() {
		uaaClient = FakeUAAClient{userGUID: "user-guid", userName: "instance-guid"}
		cfClient = mocks.PAASClient{}
		credentialSender = FakeCredentialSender{link: "https://fugacious.18f.gov/m/42"}
		broker = DeployerAccountBroker{
			uaaClient:        &uaaClient,
			cfClient:         &cfClient,
			credentialSender: &credentialSender,
			logger:           lagertest.NewTestLogger("broker-test"),
			generatePassword: func(int) string {
				return "password"
			},
			config: Config{
				EmailAddress:         "fake@fake.org",
				PasswordLength:       32,
				AccessTokenValidity:  600,
				RefreshTokenValidity: 86400,
			},
		}
	})

	Describe("uaa client", func() {
		Describe("provision", func() {
			It("returns a provision service spec", func() {
				credentialSender.On("Send", "Client ID: instance-guid\nClient Secret: password")
				uaaClient.On("CreateClient", Client{
					ID:                   "instance-guid",
					AuthorizedGrantTypes: []string{"authorization_code", "refresh_token"},
					Scope:                []string{"openid"},
					RedirectURI:          []string{"https://cloud.gov"},
					ClientSecret:         "password",
					AccessTokenValidity:  600,
					RefreshTokenValidity: 86400,
				}).Return(Client{ID: "client-guid"}, nil)

				spec, err := broker.Provision(
					context.Background(),
					"instance-guid",
					brokerapi.ProvisionDetails{
						OrganizationGUID: "org-guid",
						SpaceGUID:        "space-guid",
						ServiceID:        clientAccountGUID,
						RawParameters:    []byte(`{"redirect_uri": ["https://cloud.gov"]}`),
					},
					false,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.IsAsync).To(Equal(false))
				Expect(spec.DashboardURL).To(Equal("https://fugacious.18f.gov/m/42"))

				credentialSender.AssertExpectations(GinkgoT())
				uaaClient.AssertExpectations(GinkgoT())
			})

			It("accepts allowed scopes", func() {
				credentialSender.On("Send", "Client ID: instance-guid\nClient Secret: password")
				uaaClient.On("CreateClient", Client{
					ID:                   "instance-guid",
					AuthorizedGrantTypes: []string{"authorization_code", "refresh_token"},
					Scope:                []string{"openid", "cloud_controller.read"},
					RedirectURI:          []string{"https://cloud.gov"},
					ClientSecret:         "password",
					AccessTokenValidity:  600,
					RefreshTokenValidity: 86400,
				}).Return(Client{ID: "client-guid"}, nil)

				spec, err := broker.Provision(
					context.Background(),
					"instance-guid",
					brokerapi.ProvisionDetails{
						OrganizationGUID: "org-guid",
						SpaceGUID:        "space-guid",
						ServiceID:        clientAccountGUID,
						RawParameters:    []byte(`{"redirect_uri": ["https://cloud.gov"], "scopes": ["openid", "cloud_controller.read"]}`),
					},
					false,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.IsAsync).To(Equal(false))
				Expect(spec.DashboardURL).To(Equal("https://fugacious.18f.gov/m/42"))

				credentialSender.AssertExpectations(GinkgoT())
				uaaClient.AssertExpectations(GinkgoT())
			})

			It("rejects forbidden scopes", func() {
				spec, err := broker.Provision(
					context.Background(),
					"instance-guid",
					brokerapi.ProvisionDetails{
						OrganizationGUID: "org-guid",
						SpaceGUID:        "space-guid",
						ServiceID:        clientAccountGUID,
						RawParameters:    []byte(`{"redirect_uri": ["https://cloud.gov"], "scopes": ["cloud_controller.write"]}`),
					},
					false,
				)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Scope(s) not permitted: cloud_controller.write"))
				Expect(spec.IsAsync).To(Equal(false))
			})
		})

		Describe("deprovision", func() {
			It("returns a deprovision service spec", func() {
				uaaClient.On("DeleteClient", "instance-guid").Return(nil)

				spec, err := broker.Deprovision(
					context.Background(),
					"instance-guid",
					brokerapi.DeprovisionDetails{
						ServiceID: clientAccountGUID,
					},
					false,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.IsAsync).To(Equal(false))

				uaaClient.AssertExpectations(GinkgoT())
			})
		})
	})

	Describe("uaa user", func() {
		Describe("provision", func() {
			It("returns a provision service spec for space-deployer", func() {
				credentialSender.On("Send", "Username: instance-guid\nPassword: password")
				uaaClient.On("CreateUser", User{
					UserName: "instance-guid",
					Password: "password",
					Emails: []Email{{
						Value:   "fake@fake.org",
						Primary: true,
					}},
				}).Return(User{ID: "user-guid"}, nil)
				cfClient.On("CreateUser", cfclient.UserRequest{Guid: "user-guid"}).Return(cfclient.User{Guid: "user-guid"}, nil)
				cfClient.On("AssociateOrgUserByUsername", "org-guid", "instance-guid").Return(cfclient.Org{}, nil)
				cfClient.On("AssociateSpaceDeveloperByUsername", "space-guid", "instance-guid").Return(cfclient.Space{}, nil)

				spec, err := broker.Provision(
					context.Background(),
					"instance-guid",
					brokerapi.ProvisionDetails{
						OrganizationGUID: "org-guid",
						SpaceGUID:        "space-guid",
						ServiceID:        userAccountGUID,
						PlanID:           deployerGUID,
					},
					false,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.IsAsync).To(Equal(false))
				Expect(spec.DashboardURL).To(Equal("https://fugacious.18f.gov/m/42"))

				credentialSender.AssertExpectations(GinkgoT())
				uaaClient.AssertExpectations(GinkgoT())
				cfClient.AssertExpectations(GinkgoT())
			})

			It("returns a provision service spec for space-auditor", func() {
				credentialSender.On("Send", "Username: instance-guid\nPassword: password")
				uaaClient.On("CreateUser", User{
					UserName: "instance-guid",
					Password: "password",
					Emails: []Email{{
						Value:   "fake@fake.org",
						Primary: true,
					}},
				}).Return(User{ID: "user-guid"}, nil)
				cfClient.On("CreateUser", cfclient.UserRequest{Guid: "user-guid"}).Return(cfclient.User{Guid: "user-guid"}, nil)
				cfClient.On("AssociateOrgAuditorByUsername", "org-guid", "instance-guid").Return(cfclient.Org{}, nil)
				cfClient.On("AssociateSpaceAuditorByUsername", "space-guid", "instance-guid").Return(cfclient.Space{}, nil)

				spec, err := broker.Provision(
					context.Background(),
					"instance-guid",
					brokerapi.ProvisionDetails{
						OrganizationGUID: "org-guid",
						SpaceGUID:        "space-guid",
						ServiceID:        userAccountGUID,
						PlanID:           auditorGUID,
					},
					false,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.IsAsync).To(Equal(false))
				Expect(spec.DashboardURL).To(Equal("https://fugacious.18f.gov/m/42"))

				credentialSender.AssertExpectations(GinkgoT())
				uaaClient.AssertExpectations(GinkgoT())
				cfClient.AssertExpectations(GinkgoT())
			})
		})

		Describe("deprovision", func() {
			It("returns a deprovision service spec", func() {
				uaaClient.On("GetUser", "instance-guid").Return(User{ID: "user-guid"}, nil)
				uaaClient.On("DeleteUser", "user-guid").Return(nil)
				cfClient.On("DeleteUser", "user-guid").Return(nil)

				spec, err := broker.Deprovision(
					context.Background(),
					"instance-guid",
					brokerapi.DeprovisionDetails{
						ServiceID: userAccountGUID,
					},
					false,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.IsAsync).To(Equal(false))

				uaaClient.AssertExpectations(GinkgoT())
				cfClient.AssertExpectations(GinkgoT())
			})
		})
	})
})
