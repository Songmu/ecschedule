package ecschedule

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// GetAWSAccountID returns id
func GetAWSAccountID(conf aws.Config) (string, error) {
	svc := sts.NewFromConfig(conf)
	input := &sts.GetCallerIdentityInput{}
	result, err := svc.GetCallerIdentity(context.Background(), input)
	if err != nil {
		return "", err
	}
	if result.Account == nil {
		return "", fmt.Errorf("failed to get AWS AccountID")
	}
	return *result.Account, nil
}
