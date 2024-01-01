package ecschedule

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroups"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroups/types"
	"golang.org/x/exp/slices"
)

type Query struct {
	ResourceTypeFIlters []string    `json:"ResourceTypeFilters"`
	TagFilters          []TagFilter `json:"TagFilters"`
}

type TagFilter struct {
	Key    string   `json:"Key"`
	Values []string `json:"Values"`
}

// Extract the Rule associated with trackingId and extract those that are not included in ruleNames.
func extractOrphanedRules(ctx context.Context, awsConf aws.Config, base *BaseConfig, ruleNames []string) ([]*Rule, error) {
	trackedRuleNames, err := listTrackedRules(ctx, awsConf, base.TrackingID)
	if err != nil {
		return nil, err
	}

	var orphanedRuleNames []string
	for _, trackedRuleName := range trackedRuleNames {
		if !slices.Contains(ruleNames, trackedRuleName) {
			orphanedRuleNames = append(orphanedRuleNames, trackedRuleName)
		}
	}

	var orphanedRules []*Rule
	for _, orphanedRuleName := range orphanedRuleNames {
		orphanedRule, err := NewRuleFromRemote(ctx, awsConf, base, orphanedRuleName)
		if err != nil {
			return nil, err
		}
		orphanedRules = append(orphanedRules, orphanedRule)
	}

	return orphanedRules, nil
}

// Using the SearchResources API of the AWS Resource Groups service, extract the Rule with
// the following tags from `AWS::Events::Rule`.
// - Key: 'ecschedule:tracking-id'
// - Value: base.TrackingId
func listTrackedRules(ctx context.Context, awsConf aws.Config, trackingId string) ([]string, error) {
	svc := resourcegroups.NewFromConfig(awsConf, func(o *resourcegroups.Options) {
		o.Region = awsConf.Region
	})
	q := Query{
		ResourceTypeFIlters: []string{"AWS::Events::Rule"},
		TagFilters: []TagFilter{
			{
				Key:    "ecschedule:tracking-id",
				Values: []string{trackingId},
			},
		},
	}
	queryBytes, err := json.Marshal(q)
	if err != nil {
		return []string{}, err
	}

	input := &resourcegroups.SearchResourcesInput{
		ResourceQuery: &types.ResourceQuery{
			Type:  types.QueryTypeTagFilters10,
			Query: aws.String(string(queryBytes)),
		},
	}
	result, err := svc.SearchResources(ctx, input)
	if err != nil {
		return []string{}, err
	}
	if result.ResourceIdentifiers == nil {
		return []string{}, nil
	}

	var ruleNames []string
	for _, identifier := range result.ResourceIdentifiers {
		arn := *identifier.ResourceArn
		arnElements := strings.Split(arn, "/")
		ruleName := arnElements[len(arnElements)-1]
		ruleNames = append(ruleNames, ruleName)
	}

	return ruleNames, nil
}
