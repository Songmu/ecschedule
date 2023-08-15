package ecschedule

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/resourcegroups"
	"golang.org/x/exp/slices"
)

type Query struct {
	ResourceTypeFIlters []string `json:"ResourceTypeFilters"`
	TagFilters 					[]TagFilter    `json:"TagFilters"`
}

type TagFilter struct {
	Key		  string   `json:"Key"`
	Values	[]string `json:"Values"`
}

// Extract the Rule associated with trackingId and extract those that are not included in ruleNames.
func extractOrphanedRules(ctx context.Context, sess *session.Session, base *BaseConfig, ruleNames []string) ([]*Rule, error) {
	trackedRuleNames, err := listTrackedRules(ctx, sess, base.Cluster)
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
		orphanedRule, err := NewRuleFromRemote(ctx, sess, base, orphanedRuleName)
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
// - Value: tracking_id
func listTrackedRules(ctx context.Context, sess *session.Session, trackingId string) ([]string, error) {
	svc := resourcegroups.New(sess)
	q := Query{
		ResourceTypeFIlters: []string{"AWS::Events::Rule"},
		TagFilters: []TagFilter{
			{
				Key: "ecschedule:tracking-id",
				Values: []string{trackingId},
			},
		},
	}
	queryBytes, err := json.Marshal(q)
	if err != nil {
		return []string{}, err
	}

	input := &resourcegroups.SearchResourcesInput{
		ResourceQuery: &resourcegroups.ResourceQuery{
			Type: aws.String(resourcegroups.QueryTypeTagFilters10),
			Query: aws.String(string(queryBytes)),
		},
	}
	result, err := svc.SearchResourcesWithContext(ctx, input)
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
		ruleName := arnElements[len(arnElements) - 1]
		ruleNames = append(ruleNames, ruleName)
	}

	return ruleNames, nil
}
