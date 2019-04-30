package aws

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"golang.org/x/xerrors"
)

// LocalHostname returns the EC2 private hostname
// for the instance.
func LocalHostname() (string, error) {
	// Query the address to read EC2 local (private) hostname.
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html
	resp, err := http.Get("http://169.254.169.254/latest/meta-data/local-hostname")
	if err != nil {
		return "", xerrors.Errorf("querying ec2 local hostname: %w", err)
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	return string(b), err
}

type instanceIdentity struct {
	InstanceID string `json:"instanceId"`
	Region     string `json:"region"`
}

// getIdentity queries AWS's metadata API to retrieve the instance's
// identity document.
func getIdentity(client *http.Client) (doc instanceIdentity, err error) {
	// Query the address to read EC2 instance identity document
	// See: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
	resp, err := client.Get("http://169.254.169.254/latest/dynamic/instance-identity/document")
	if err != nil {
		return doc, xerrors.Errorf("querying ec2 instance-identity document: %w", err)
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&doc)
	return doc, err
}

// Store initializes a ParameterStore based on
// the current EC2 instance's identity
func Store() (*ParameterStore, string, string, error) {
	identity, err := getIdentity(http.DefaultClient)
	if err != nil {
		return nil, "", "", err
	}

	// Create an AWS API session
	sess := session.New(&aws.Config{Region: &identity.Region})
	cred := credentials.NewChainCredentials(
		[]credentials.Provider{
			&ec2rolecreds.EC2RoleProvider{
				Client: ec2metadata.New(sess),
			},
			&credentials.EnvProvider{},
		})
	// Checking for invalid or missing credentials
	_, err = cred.Get()
	if err != nil {
		return nil, "", "", err
	}
	sess.Config.Credentials = cred

	// Query the AWS API to retrieve our stack from the
	// instance's tags.
	svc := ec2.New(sess)
	res, err := svc.DescribeTags(&ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("resource-id"),
				Values: []*string{aws.String(identity.InstanceID)},
			},
			{
				Name:   aws.String("key"),
				Values: []*string{aws.String("Stack")},
			},
		},
	})
	if err != nil {
		return nil, "", "", err
	}
	if len(res.Tags) != 1 {
		return nil, "", "", fmt.Errorf("querying aws stack tag: found %d tags", len(res.Tags))
	}
	stack := *res.Tags[0].Value

	ps := &ParameterStore{
		client: ssm.New(sess),
	}
	return ps, identity.Region, stack, nil
}

// Region queries the instance metadata API for
// the instance's region.
func Region() (string, error) {
	identity, err := getIdentity(http.DefaultClient)
	if err != nil {
		return "", xerrors.Errorf("querying ec2 region: %w", err)
	}
	return identity.Region, nil
}

// ParameterStore contains an SSM client and parameter path prefix
type ParameterStore struct {
	client     *ssm.SSM
	PathPrefix string // e.g. /orstage/ledgerd/env/
}

// GetString gets a Parameter Store parameter's value as a string.
// If the parameter does not exist or if there is an error, we return fallback.
func (ps *ParameterStore) GetString(name, fallback string) string {
	param := &ssm.GetParameterInput{
		Name:           aws.String(path.Join(ps.PathPrefix, name)),
		WithDecryption: aws.Bool(true),
	}
	response, err := ps.client.GetParameter(param)
	if err != nil {
		return fallback
	}
	return *(response.Parameter.Value)
}

// GetBytes gets a Parameter Store parameter's value as bytes.
// If the parameter does not exist or if there is an error, we return fallback.
func (ps *ParameterStore) GetBytes(name string, fallback []byte) []byte {
	str := ps.GetString(name, "")
	if str == "" {
		return fallback
	}
	param, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return fallback
	}
	return param
}

// GetInt gets a Parameter Store parameter's value as an int.
// If the parameter does not exist or if there is an error, we return fallback.
func (ps *ParameterStore) GetInt(name string, fallback int) int {
	str := ps.GetString(name, "")
	if str == "" {
		return fallback
	}
	param, err := strconv.Atoi(str)
	if err != nil {
		return fallback
	}
	return param
}

// GetDuration gets a Parameter Store parameter's value as a duration.
// If the parameter does not exist or if there is an error, we return fallback.
func (ps *ParameterStore) GetDuration(name string, fallback time.Duration) time.Duration {
	str := ps.GetString(name, "")
	if str == "" {
		return fallback
	}
	param, err := time.ParseDuration(str)
	if err != nil {
		return fallback
	}
	return param
}

// GetBool gets a Parameter Store parameter's value as a bool.
// If the parameter does not exist or if there is an error, we return fallback.
func (ps *ParameterStore) GetBool(name string, fallback bool) bool {
	str := ps.GetString(name, "")
	if str == "" {
		return fallback
	}
	param, err := strconv.ParseBool(str)
	if err != nil {
		return fallback
	}
	return param
}
