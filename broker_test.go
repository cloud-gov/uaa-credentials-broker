package main

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/pivotal-cf/brokerapi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/cloud-gov/uaa-credentials-broker/mocks"
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
	args := c.Called(clientID)
	return args.Error(0)
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

var _ = Describe("broker", func() {
	var (
		uaaClient FakeUAAClient
		cfClient  mocks.PAASClient
		broker    DeployerAccountBroker
	)

	BeforeEach(func() {
		uaaClient = FakeUAAClient{userGUID: "user-guid", userName: "binding-guid"}
		cfClient = mocks.PAASClient{}
		broker = DeployerAccountBroker{
			uaaClient: &uaaClient,
			cfClient:  &cfClient,
			logger:    lagertest.NewTestLogger("broker-test"),
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
		Describe("parse options", func() {
			It("returns error options when no parameters are specified", func() {
				options, err := parseBindOptions(brokerapi.BindDetails{
					RawParameters: []byte(``),
				})
				Expect(err).To(HaveOccurred())
				Expect(options).To(Equal(BindOptions{}))
			})

			It("returns error options when no redirect URI specified", func() {
				options, err := parseBindOptions(brokerapi.BindDetails{
					RawParameters: []byte(`{"redirect_uri":[]}`),
				})
				Expect(err).To(HaveOccurred())
				Expect(options).To(Equal(BindOptions{
					RedirectURI: []string{},
				}))
			})

			It("returns options with redirect URI", func() {
				options, err := parseBindOptions(brokerapi.BindDetails{
					RawParameters: []byte(`{"redirect_uri":["example.com"]}`),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(options).To(Equal(BindOptions{
					RedirectURI: []string{"example.com"},
				}))
			})

			It("returns options with scopes", func() {
				options, err := parseBindOptions(brokerapi.BindDetails{
					RawParameters: []byte(`{"redirect_uri":["example.com"], "scopes":["scope1"]}`),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(options).To(Equal(BindOptions{
					RedirectURI: []string{"example.com"},
					Scopes:      []string{"scope1"},
				}))
			})

			It("returns options with allowpublic", func() {
				options, err := parseBindOptions(brokerapi.BindDetails{
					RawParameters: []byte(`{"redirect_uri":["example.com"], "allowpublic": true}`),
				})
				Expect(err).NotTo(HaveOccurred())
				allowPublicTrue := true
				Expect(options).To(Equal(BindOptions{
					RedirectURI: []string{"example.com"},
					AllowPublic: &allowPublicTrue,
				}))
			})
		})

		Describe("provision", func() {
			It("returns a binding", func() {
				uaaClient.On("CreateClient", Client{
					ID:                   "binding-guid",
					AuthorizedGrantTypes: []string{"authorization_code", "refresh_token"},
					Scope:                []string{"openid"},
					RedirectURI:          []string{"https://cloud.gov"},
					ClientSecret:         "password",
					AccessTokenValidity:  600,
					RefreshTokenValidity: 86400,
				}).Return(Client{ID: "client-guid"}, nil)

				_, err := broker.Bind(
					context.Background(),
					"instance-guid",
					"binding-guid",
					brokerapi.BindDetails{
						AppGUID:       "app-guid",
						ServiceID:     clientAccountGUID,
						RawParameters: []byte(`{"redirect_uri": ["https://cloud.gov"]}`),
					},
				)
				Expect(err).NotTo(HaveOccurred())
				cfClient.AssertExpectations(GinkgoT())
				uaaClient.AssertExpectations(GinkgoT())
			})

			It("errors if params missing", func() {
				uaaClient.On("CreateClient", Client{
					ID:                   "binding-guid",
					AuthorizedGrantTypes: []string{"authorization_code", "refresh_token"},
					Scope:                []string{"openid"},
					RedirectURI:          []string{"https://cloud.gov"},
					ClientSecret:         "password",
					AccessTokenValidity:  600,
					RefreshTokenValidity: 86400,
				}).Return(Client{ID: "client-guid"}, nil)

				_, err := broker.Bind(
					context.Background(),
					"instance-guid",
					"binding-guid",
					brokerapi.BindDetails{
						AppGUID:   "app-guid",
						ServiceID: clientAccountGUID,
					},
				)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(`must pass JSON configuration with field "redirect_uri"`))
			})

			It("errors if params incomplete", func() {
				uaaClient.On("CreateClient", Client{
					ID:                   "binding-guid",
					AuthorizedGrantTypes: []string{"authorization_code", "refresh_token"},
					Scope:                []string{"openid"},
					RedirectURI:          []string{"https://cloud.gov"},
					ClientSecret:         "password",
					AccessTokenValidity:  600,
					RefreshTokenValidity: 86400,
				}).Return(Client{ID: "client-guid"}, nil)

				_, err := broker.Bind(
					context.Background(),
					"instance-guid",
					"binding-guid",
					brokerapi.BindDetails{
						AppGUID:       "app-guid",
						ServiceID:     clientAccountGUID,
						RawParameters: []byte(`{}`),
					},
				)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(`must pass field "redirect_uri"`))
			})

			It("accepts allowed scopes", func() {
				uaaClient.On("CreateClient", Client{
					ID:                   "binding-guid",
					AuthorizedGrantTypes: []string{"authorization_code", "refresh_token"},
					Scope:                []string{"openid"},
					RedirectURI:          []string{"https://cloud.gov"},
					ClientSecret:         "password",
					AccessTokenValidity:  600,
					RefreshTokenValidity: 86400,
				}).Return(Client{ID: "client-guid"}, nil)

				_, err := broker.Bind(
					context.Background(),
					"instance-guid",
					"binding-guid",
					brokerapi.BindDetails{
						AppGUID:       "app-guid",
						ServiceID:     clientAccountGUID,
						RawParameters: []byte(`{"redirect_uri": ["https://cloud.gov"], "scopes": ["openid"]}`),
					},
				)
				Expect(err).NotTo(HaveOccurred())
				cfClient.AssertExpectations(GinkgoT())
				uaaClient.AssertExpectations(GinkgoT())
			})

			It("rejects forbidden scopes", func() {
				_, err := broker.Bind(
					context.Background(),
					"instance-guid",
					"binding-guid",
					brokerapi.BindDetails{
						AppGUID:       "app-guid",
						ServiceID:     clientAccountGUID,
						RawParameters: []byte(`{"redirect_uri": ["https://cloud.gov"], "scopes": ["cloud_controller.write"]}`),
					},
				)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Scope(s) not permitted: cloud_controller.write"))
			})

			It("uses specified allowpublic value", func() {
				uaaClient.On("CreateClient", Client{
					ID:                   "binding-guid",
					AuthorizedGrantTypes: []string{"authorization_code", "refresh_token"},
					Scope:                []string{"openid"},
					RedirectURI:          []string{"https://cloud.gov"},
					ClientSecret:         "password",
					AccessTokenValidity:  600,
					RefreshTokenValidity: 86400,
					AllowPublic:          true,
				}).Return(Client{ID: "client-guid"}, nil)

				_, err := broker.Bind(
					context.Background(),
					"instance-guid",
					"binding-guid",
					brokerapi.BindDetails{
						AppGUID:       "app-guid",
						ServiceID:     clientAccountGUID,
						RawParameters: []byte(`{"redirect_uri": ["https://cloud.gov"], "scopes": ["openid"], "allowpublic": true}`),
					},
				)
				Expect(err).NotTo(HaveOccurred())
				cfClient.AssertExpectations(GinkgoT())
				uaaClient.AssertExpectations(GinkgoT())
			})
		})

		Describe("unbind", func() {
			It("does not return an error", func() {
				uaaClient.On("DeleteClient", "binding-guid").Return(nil)

				err := broker.Unbind(
					context.Background(),
					"instance-guid",
					"binding-guid",
					brokerapi.UnbindDetails{
						ServiceID: clientAccountGUID,
					},
				)
				Expect(err).NotTo(HaveOccurred())
				uaaClient.AssertExpectations(GinkgoT())
			})
		})

		Describe("deprovision", func() {
			It("does not return an error", func() {
				uaaClient.On("DeleteClient", "binding-guid").Return(nil)

				err := broker.Unbind(
					context.Background(),
					"instance-guid",
					"binding-guid",
					brokerapi.UnbindDetails{
						ServiceID: clientAccountGUID,
					},
				)
				Expect(err).NotTo(HaveOccurred())
				uaaClient.AssertExpectations(GinkgoT())
			})
		})

		It("does not return an error for a 404 response on deletion", func() {
			uaaClient.On("DeleteClient", "instance-guid2").Return(fmt.Errorf("Expected status 200; got: %d", 404))

			_, err := broker.Deprovision(
				context.Background(),
				"instance-guid2",
				brokerapi.DeprovisionDetails{
					ServiceID: clientAccountGUID,
				},
				false,
			)
			Expect(err).NotTo(HaveOccurred())
			uaaClient.AssertExpectations(GinkgoT())
		})

		It("does return an error for a response other than 200/404 on deletion", func() {
			uaaClient.On("DeleteClient", "instance-guid3").Return(fmt.Errorf("Expected status 200; got: %d", 500))

			_, err := broker.Deprovision(
				context.Background(),
				"instance-guid3",
				brokerapi.DeprovisionDetails{
					ServiceID: clientAccountGUID,
				},
				false,
			)
			Expect(err).To(HaveOccurred())
			uaaClient.AssertExpectations(GinkgoT())
		})
	})

	Describe("uaa user", func() {
		Describe("provision", func() {
			It("returns a provision service spec for space-deployer", func() {
				cfClient.On("ServiceInstanceByGuid", "instance-guid").Return(cfclient.ServiceInstance{SpaceGuid: "space-guid"}, nil)
				cfClient.On("GetSpaceByGuid", "space-guid").Return(cfclient.Space{OrganizationGuid: "org-guid"}, nil)
				uaaClient.On("CreateUser", User{
					UserName: "binding-guid",
					Password: "password",
					Emails: []Email{{
						Value:   "fake@fake.org",
						Primary: true,
					}},
				}).Return(User{ID: "user-guid"}, nil)
				cfClient.On("CreateUser", cfclient.UserRequest{Guid: "user-guid"}).Return(cfclient.User{Guid: "user-guid"}, nil)
				cfClient.On("AssociateOrgUserByUsername", "org-guid", "binding-guid").Return(cfclient.Org{}, nil)
				cfClient.On("AssociateSpaceDeveloperByUsername", "space-guid", "binding-guid").Return(cfclient.Space{}, nil)

				_, err := broker.Bind(
					context.Background(),
					"instance-guid",
					"binding-guid",
					brokerapi.BindDetails{
						AppGUID:   "app-guid",
						ServiceID: userAccountGUID,
						PlanID:    deployerGUID,
					},
				)
				Expect(err).NotTo(HaveOccurred())
				uaaClient.AssertExpectations(GinkgoT())
				cfClient.AssertExpectations(GinkgoT())
			})

			It("returns a provision service spec for space-auditor", func() {
				cfClient.On("ServiceInstanceByGuid", "instance-guid").Return(cfclient.ServiceInstance{SpaceGuid: "space-guid"}, nil)
				cfClient.On("GetSpaceByGuid", "space-guid").Return(cfclient.Space{OrganizationGuid: "org-guid"}, nil)
				uaaClient.On("CreateUser", User{
					UserName: "binding-guid",
					Password: "password",
					Emails: []Email{{
						Value:   "fake@fake.org",
						Primary: true,
					}},
				}).Return(User{ID: "user-guid"}, nil)
				cfClient.On("CreateUser", cfclient.UserRequest{Guid: "user-guid"}).Return(cfclient.User{Guid: "user-guid"}, nil)
				cfClient.On("AssociateOrgUserByUsername", "org-guid", "binding-guid").Return(cfclient.Org{}, nil)
				cfClient.On("AssociateSpaceAuditorByUsername", "space-guid", "binding-guid").Return(cfclient.Space{}, nil)

				_, err := broker.Bind(
					context.Background(),
					"instance-guid",
					"binding-guid",
					brokerapi.BindDetails{
						AppGUID:   "app-guid",
						ServiceID: userAccountGUID,
						PlanID:    auditorGUID,
					},
				)
				Expect(err).NotTo(HaveOccurred())
				uaaClient.AssertExpectations(GinkgoT())
				cfClient.AssertExpectations(GinkgoT())
			})
		})

		Describe("deprovision", func() {
			It("returns a deprovision service spec", func() {
				uaaClient.On("GetUser", "binding-guid").Return(User{ID: "user-guid"}, nil)
				uaaClient.On("DeleteUser", "user-guid").Return(nil)
				cfClient.On("DeleteUser", "user-guid").Return(nil)

				err := broker.Unbind(
					context.Background(),
					"instance-guid",
					"binding-guid",
					brokerapi.UnbindDetails{
						ServiceID: userAccountGUID,
					},
				)
				Expect(err).NotTo(HaveOccurred())
				uaaClient.AssertExpectations(GinkgoT())
				cfClient.AssertExpectations(GinkgoT())
			})
		})
	})
})
