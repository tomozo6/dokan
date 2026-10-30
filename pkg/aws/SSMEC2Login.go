package aws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func SSMEC2Login(c *ssm.Client, ec2ID *string, region *string, profile *string) error {
	sessionInput := &ssm.StartSessionInput{
		DocumentName: aws.String("AWS-StartInteractiveCommand"),
		Parameters: map[string][]string{
			"command": {"bash"},
		},
		Target: ec2ID,
	}

	return runSSMSession(c, sessionInput, *region, *profile)
}
