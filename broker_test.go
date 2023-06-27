package main

import (
	"context"
	"reflect"
	"testing"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/pivotal-cf/brokerapi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/cloudfoundry-community/uaa-credentials-broker/mocks"
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
						RawParameters: []byte(`{"redirect_uri": ["https://cloud.gov"], "scopes": ["openid"], "allow_public": true}`),
					},
				)
				Expect(err).NotTo(HaveOccurred())
				cfClient.AssertExpectations(GinkgoT())
				uaaClient.AssertExpectations(GinkgoT())
			})
		})

		Describe("deprovision", func() {
			It("returns a deprovision service spec", func() {
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

func TestParseBindOptions(t *testing.T) {
	allowpublicTrue := true
	testCases := map[string]struct {
		bindDetails     brokerapi.BindDetails
		expectedOptions BindOptions
		expectErr       bool
	}{
		"no input": {
			bindDetails: brokerapi.BindDetails{
				RawParameters: []byte(``),
			},
			expectedOptions: BindOptions{},
			expectErr:       true,
		},
		"no redirect uri specified": {
			bindDetails: brokerapi.BindDetails{
				RawParameters: []byte(`{"redirect_uri":[]}`),
			},
			expectedOptions: BindOptions{
				RedirectURI: []string{},
			},
			expectErr: true,
		},
		"specify redirect URI": {
			bindDetails: brokerapi.BindDetails{
				RawParameters: []byte(`{"redirect_uri":["example.com"]}`),
			},
			expectedOptions: BindOptions{
				RedirectURI: []string{"example.com"},
			},
		},
		"specify scopes": {
			bindDetails: brokerapi.BindDetails{
				RawParameters: []byte(`{"redirect_uri":["example.com"], "scopes":["scope1"]}`),
			},
			expectedOptions: BindOptions{
				RedirectURI: []string{"example.com"},
				Scopes:      []string{"scope1"},
			},
		},
		"specify allowpublic": {
			bindDetails: brokerapi.BindDetails{
				RawParameters: []byte(`{"redirect_uri":["example.com"], "allow_public": true}`),
			},
			expectedOptions: BindOptions{
				RedirectURI: []string{"example.com"},
				AllowPublic: &allowpublicTrue,
			},
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			options, err := parseBindOptions(test.bindDetails)
			if err != nil && !test.expectErr {
				t.Fatalf("unexpected error: %s", err)
			}
			if !reflect.DeepEqual(test.expectedOptions, options) {
				t.Errorf("expected: %#v, got: %#v", test.expectedOptions, options)
			}
		})
	}
}
