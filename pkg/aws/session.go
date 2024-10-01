package aws

import (
	"errors"
	"time"

	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	defaultMaxRetries = 10
)

type SessionParameters struct {
	AccessKey  string
	SecretKey  string
	Region     string
	IamRole    string
	IamSession string
	MaxRetries int
}

func NewAwsSession(parameters *SessionParameters) (*session.Session, error) {
	if parameters.Region == "" {
		return nil, errors.New("Missing aws region (required).")
	}

	if parameters.MaxRetries == 0 {
		parameters.MaxRetries = defaultMaxRetries
	}

	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region:     aws.String(parameters.Region),
			MaxRetries: &parameters.MaxRetries,
		},
		// Support MFA when authing using assumed roles.
		SharedConfigState:       session.SharedConfigEnable,
		AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
	})

	if err != nil {
		log.Fatalf("unable to create new AWS session because of %s", err)
	}

	if parameters.AccessKey != "" && parameters.SecretKey != "" {
		sess = session.New(&aws.Config{
			Region:      aws.String(parameters.Region),
			MaxRetries:  &parameters.MaxRetries,
			Credentials: credentials.NewStaticCredentials(parameters.AccessKey, parameters.SecretKey, ""),
		})
	}

	if parameters.IamRole != "" {
		creds := assumeRoleCredentials(sess, parameters.IamRole, parameters.IamSession)
		sess.Config.Credentials = creds
	}

	return sess, nil
}

func assumeRoleCredentials(sess *session.Session, iamRole, iamSession string) *credentials.Credentials {
	if iamSession == "" {
		iamSession = "default"
	}

	creds := stscreds.NewCredentials(sess, iamRole, func(o *stscreds.AssumeRoleProvider) {
		o.Duration = time.Hour
		o.ExpiryWindow = 5 * time.Minute
		o.RoleSessionName = iamSession
	})
	return creds
}
