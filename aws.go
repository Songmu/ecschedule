package ecsched

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
)

func NewAWSSession(region string) (*session.Session, error) {
	return session.NewSession(&aws.Config{Region: aws.String(region)})
}

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
