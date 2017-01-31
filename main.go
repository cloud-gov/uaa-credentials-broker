package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/kelseyhightower/envconfig"
	"github.com/pivotal-cf/brokerapi"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type Config struct {
	UAAAddress           string `envconfig:"uaa_address" required:"true"`
	UAAClientID          string `envconfig:"uaa_client_id" required:"true"`
	UAAClientSecret      string `envconfig:"uaa_client_secret" required:"true"`
	UAAZone              string `envconfig:"uaa_zone" default:"uaa"`
	CFAddress            string `envconfig:"cf_address" required:"true"`
	BrokerUsername       string `envconfig:"broker_username" required:"true"`
	BrokerPassword       string `envconfig:"broker_password" required:"true"`
	PasswordLength       int    `envconfig:"password_length" default:"32"`
	EmailAddress         string `envconfig:"email_address" required:"true"`
	FugaciousAddress     string `envconfig:"fugacious_address" required:"true"`
	FugaciousHours       int    `envconfig:"fugacious_hours" default:"2"`
	FugaciousMaxViews    int    `envconfig:"fugacious_max_views" default:"2"`
	AccessTokenValidity  int    `envconfig:"access_token_validity" default:"600"`
	RefreshTokenValidity int    `envconfig:"refresh_token_validity" default:"86400"`
	Port                 string `envconfig:"port" default:"3000"`
}

func NewClient(config Config) *http.Client {
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, http.DefaultClient)
	cfg := &clientcredentials.Config{
		ClientID:     config.UAAClientID,
		ClientSecret: config.UAAClientSecret,
		TokenURL:     fmt.Sprintf("%s/oauth/token", config.UAAAddress),
	}
	return cfg.Client(ctx)
}

func main() {
	logger := lager.NewLogger("uaa-credentials-broker")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.INFO))

	config := Config{}
	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatalf("", err)
	}

	client := NewClient(config)

	broker := DeployerAccountBroker{
		logger: logger,
		uaaClient: &UAAClient{
			logger:   logger,
			endpoint: config.UAAAddress,
			zone:     config.UAAZone,
			client:   client,
		},
		cfClient: &CFClient{
			logger:   logger,
			endpoint: config.CFAddress,
			client:   client,
		},
		credentialSender: FugaciousCredentialSender{
			endpoint: config.FugaciousAddress,
			hours:    config.FugaciousHours,
			maxViews: config.FugaciousMaxViews,
		},
		generatePassword: GenerateSecurePassword,
		config:           config,
	}
	credentials := brokerapi.BrokerCredentials{
		Username: config.BrokerUsername,
		Password: config.BrokerPassword,
	}

	brokerAPI := brokerapi.New(&broker, logger, credentials)
	http.Handle("/", brokerAPI)
	http.ListenAndServe(fmt.Sprintf(":%s", config.Port), nil)
}
