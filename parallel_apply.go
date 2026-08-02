package ecschedule

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchevents"
	"github.com/goccy/go-yaml"
)

// parallelRetryMaxAttempts raises the SDK retry budget for the clients
// built by runParallelApply only: at parallel > 1, EventBridge write
// throttling is the expected steady state for large rule sets and the
// default 3 attempts exhaust quickly. The sequential path keeps SDK
// defaults.
const parallelRetryMaxAttempts = 10

// runParallelApply is the three-phase apply path used for real apply
// with -parallel > 1: plan every rule (read-only), execute only the
// changed ones (continue-on-error), then print a summary.
//
// It returns nil ONLY when every rule succeeded (applied or no change)
// and the run was not interrupted — callers use this as the -prune gate.
func runParallelApply(ctx context.Context, awsConf aws.Config, c *Config, ruleNames []string, parallel int, format diffFormat) error {
	// Resolve rules and build one client per region, eagerly: the region
	// set is known up front, so the map is read-only once workers start.
	rules := make(map[string]*Rule, len(ruleNames))
	clients := map[string]*cloudwatchevents.Client{}
	for _, name := range ruleNames {
		ru := c.GetRuleByName(name)
		if ru == nil {
			return fmt.Errorf("no rules found for %s", name)
		}
		rules[name] = ru
		region := ru.Region
		if _, ok := clients[region]; !ok {
			clients[region] = cloudwatchevents.NewFromConfig(awsConf, func(o *cloudwatchevents.Options) {
				o.Region = region
				o.RetryMaxAttempts = parallelRetryMaxAttempts
			})
		}
	}

	// Phase 1: plan (read-only). All rules are always planned; a real
	// error lists every failure and blocks all writes. Interruption is
	// classified separately so Ctrl-C doesn't masquerade as a wall of
	// validation errors.
	plans := make(map[string]*applyPlan, len(ruleNames))
	var planErrs []jobOutcome[*applyPlan]
	interrupted := false
	for out := range executeJobsInParallelContinueOnError(ctx, ruleNames, parallel,
		func(ctx context.Context, name string) (*applyPlan, error) {
			ru := rules[name]
			return ru.plan(ctx, awsConf, clients[ru.Region], format)
		}) {
		switch {
		case out.Skipped || (out.Err != nil && errors.Is(out.Err, context.Canceled)):
			interrupted = true
		case out.Err != nil:
			planErrs = append(planErrs, out)
		default:
			plans[out.Name] = out.Result
		}
	}
	if len(planErrs) > 0 {
		sort.Slice(planErrs, func(i, j int) bool { return planErrs[i].Index < planErrs[j].Index })
		for _, out := range planErrs {
			log.Printf("❌ rule %q: %s", out.Name, out.Err)
		}
		return fmt.Errorf("%d rule(s) failed validation/planning; no writes were performed", len(planErrs))
	}
	if interrupted {
		log.Println("interrupted during planning; no writes were performed")
		return errors.New("interrupted: planning canceled")
	}

	// Classify: no-change rules are reported and excluded from Phase 2.
	var toApply []string
	noChange := 0
	for _, name := range ruleNames {
		if plans[name].hasChange {
			toApply = append(toApply, name)
		} else {
			log.Printf("💡 rule %q: skip applying. no differences", name)
			noChange++
		}
	}

	// Phase 2: execute (writes). Started rules always finish (execute
	// shields the write sequence internally); on the first signal only
	// admission stops.
	stopNotice := context.AfterFunc(ctx, func() {
		log.Printf("⚠️ interrupted: waiting for in-flight rule(s) to finish (max %s); press Ctrl-C again to force quit", perRuleApplyTimeout)
	})
	defer stopNotice()

	applied := 0
	var failed []jobOutcome[string]
	var notStartedOut []jobOutcome[string]
	for out := range executeJobsInParallelContinueOnError(ctx, toApply, parallel,
		func(ctx context.Context, name string) (string, error) {
			ru := rules[name]
			log.Printf("applying rule %q", name)
			if err := ru.execute(ctx, clients[ru.Region]); err != nil {
				return "", err
			}
			// Display-time mutation: strictly after this rule's writes.
			for _, v := range ru.ContainerOverrides {
				v.Environment = nil
			}
			bs, _ := yaml.Marshal(ru)
			return string(bs), nil
		}) {
		switch {
		case out.Skipped:
			notStartedOut = append(notStartedOut, out)
		case out.Err != nil:
			var tagErr *tagResourceError
			if errors.As(out.Err, &tagErr) {
				log.Printf("❌ rule %q: %s\n"+
					"⚠️ the rule was applied but tagging failed: a rule created by this run is left without the \"ecschedule:tracking-id\" tag and escapes -prune orphan detection (an already-tagged rule keeps its previous tag). Re-running apply will NOT retry the tag. Repair manually:\n"+
					"  aws events tag-resource --resource-arn %s --tags Key=ecschedule:tracking-id,Value=%s",
					out.Name, out.Err, tagErr.ruleARN, tagErr.trackingID)
			} else {
				log.Printf("❌ rule %q: %s", out.Name, out.Err)
			}
			failed = append(failed, out)
		default:
			log.Printf("✅ rule %q applied\n💡 applied changes:\n%s%s",
				out.Name, strings.TrimRight(plans[out.Name].diffOutput, "\n")+"\n", out.Result)
			applied++
		}
	}

	// Phase 3: summary. Lists are deterministic (config order via Index).
	sort.Slice(failed, func(i, j int) bool { return failed[i].Index < failed[j].Index })
	sort.Slice(notStartedOut, func(i, j int) bool { return notStartedOut[i].Index < notStartedOut[j].Index })
	notStarted := make([]string, 0, len(notStartedOut))
	for _, out := range notStartedOut {
		notStarted = append(notStarted, out.Name)
	}
	log.Print(formatApplySummary(applied, noChange, failed, notStarted))

	if len(failed) > 0 {
		return fmt.Errorf("%d rule(s) failed", len(failed))
	}
	if len(notStarted) > 0 {
		return fmt.Errorf("interrupted: %d rule(s) not started", len(notStarted))
	}
	if ctx.Err() != nil {
		return errors.New("interrupted")
	}
	return nil
}

func formatApplySummary(applied, noChange int, failed []jobOutcome[string], notStarted []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "apply summary: %d applied, %d no change, %d failed, %d not started",
		applied, noChange, len(failed), len(notStarted))
	if len(failed) > 0 {
		b.WriteString("\nfailed:")
		for _, f := range failed {
			fmt.Fprintf(&b, "\n  - %s: %s", f.Name, f.Err)
		}
	}
	if len(notStarted) > 0 {
		fmt.Fprintf(&b, "\nnot started (interrupted): %s", strings.Join(notStarted, ", "))
	}
	return b.String()
}
