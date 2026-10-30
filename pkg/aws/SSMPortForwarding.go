package aws

import (
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func SSMPortForwarding(c *ssm.Client, bastionID *string, dbHost *string, dbPort *int32, localPort *int32, region *string, profile *string) error {
	sessionInput := &ssm.StartSessionInput{
		DocumentName: aws.String("AWS-StartPortForwardingSessionToRemoteHost"),
		Parameters: map[string][]string{
			"host":            {*dbHost},
			"portNumber":      {strconv.Itoa(int(*dbPort))},
			"localPortNumber": {strconv.Itoa(int(*localPort))},
		},
		Target: bastionID,
	}

	return runSSMSession(c, sessionInput, *region, *profile)
}
