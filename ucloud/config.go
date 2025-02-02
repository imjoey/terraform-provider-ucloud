package ucloud

import (
	"fmt"
	"github.com/ucloud/ucloud-sdk-go/external"
	"github.com/ucloud/ucloud-sdk-go/ucloud"
	"github.com/ucloud/ucloud-sdk-go/ucloud/auth"
	"github.com/ucloud/ucloud-sdk-go/ucloud/log"

	pumem "github.com/ucloud/ucloud-sdk-go/private/services/umem"
	"github.com/ucloud/ucloud-sdk-go/services/uaccount"
	"github.com/ucloud/ucloud-sdk-go/services/udb"
	"github.com/ucloud/ucloud-sdk-go/services/udisk"
	"github.com/ucloud/ucloud-sdk-go/services/udpn"
	"github.com/ucloud/ucloud-sdk-go/services/uhost"
	"github.com/ucloud/ucloud-sdk-go/services/ulb"
	"github.com/ucloud/ucloud-sdk-go/services/umem"
	"github.com/ucloud/ucloud-sdk-go/services/unet"
	"github.com/ucloud/ucloud-sdk-go/services/vpc"
)

// Config is the configuration of ucloud meta data
type Config struct {
	PublicKey             string
	PrivateKey            string
	Profile               string
	SharedCredentialsFile string

	Region     string
	ProjectId  string
	Insecure   bool
	BaseURL    string
	MaxRetries int
}

// Client will returns a client with connections for all product
func (c *Config) Client() (*UCloudClient, error) {
	var err error
	var client UCloudClient
	var cfg *ucloud.Config
	var cred *auth.Credential

	client.region = c.Region
	client.projectId = c.ProjectId

	cfgV := ucloud.NewConfig()
	cfg = &cfgV

	// set common attributes (region, project id, etc ...)
	cfg.Region = c.Region
	cfg.ProjectId = c.ProjectId

	// enable auto retry with http/connection error
	cfg.MaxRetries = c.MaxRetries
	cfg.LogLevel = log.DebugLevel
	cfg.UserAgent = "Terraform-UCloud/1.11.0"

	// excepted logging
	cfg.SetActionLevel("GetRegion", log.WarnLevel)

	// if no base url be set, get insecure http or secure https default url
	// uf base url is set, use it
	if len(c.BaseURL) == 0 && c.Insecure {
		cfg.BaseUrl = GetInsecureEndpointURL(c.Region)
	} else if len(c.BaseURL) == 0 && !c.Insecure {
		cfg.BaseUrl = GetEndpointURL(c.Region)
	} else {
		cfg.BaseUrl = c.BaseURL
	}

	if len(c.Profile) != 0 {
		// load public/private key from shared credential file
		cred, err = external.LoadUCloudCredentialFile(c.SharedCredentialsFile, c.Profile)
		if err != nil {
			return nil, fmt.Errorf("cannot load shared credential file, %s", err)
		}
	} else {
		// load public/private key from shared credential file
		credV := auth.NewCredential()
		cred = &credV
		cred.PublicKey = c.PublicKey
		cred.PrivateKey = c.PrivateKey
	}

	// initialize client connections
	client.uhostconn = uhost.NewClient(cfg, cred)
	client.unetconn = unet.NewClient(cfg, cred)
	client.ulbconn = ulb.NewClient(cfg, cred)
	client.vpcconn = vpc.NewClient(cfg, cred)
	client.uaccountconn = uaccount.NewClient(cfg, cred)
	client.udiskconn = udisk.NewClient(cfg, cred)
	client.udpnconn = udpn.NewClient(cfg, cred)
	client.udbconn = udb.NewClient(cfg, cred)
	client.umemconn = umem.NewClient(cfg, cred)

	// initialize client connections for private usage
	client.pumemconn = pumem.NewClient(cfg, cred)

	client.config = cfg
	client.credential = cred
	return &client, nil
}
