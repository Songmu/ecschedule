package ecschedule

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
)

// GetAWSAccountID returns id
func GetAWSAccountID(sess *session.Session) (string, error) {
	svc := sts.New(sess)
	input := &sts.GetCallerIdentityInput{}
	result, err := svc.GetCallerIdentityWithContext(context.Background(), input)
	if err != nil {
		return "", err
	}
	if result.Account == nil {
		return "", fmt.Errorf("failed to get AWS AccountID")
	}
	return *result.Account, nil
}
