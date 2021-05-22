package pipelineascode

import (
	"context"
	"strings"

	"github.com/google/go-github/v34/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"sigs.k8s.io/yaml"
)

var okToTestCommentRegexp = `(^|\n)/ok-to-test(\r\n|$)`

// OwnersConfig prow owner, only supporting approvers or reviewers in yaml
type OwnersConfig struct {
	Approvers []string `json:"approvers,omitempty"`
	Reviewers []string `json:"reviewers,omitempty"`
}

// allowedOkToTestFromAnOwner Goes on evry comments in a pull-request and sess
// if there is a /ok-to-test in there running an aclCheck again on the commment
// Sender if she is an OWNER and then allow it to run CI.
// TODO: pull out the github logic from there in an agnostic way.
func allowedOkToTestFromAnOwner(ctx context.Context, cs *cli.Clients, runinfo *webvcs.RunInfo) (bool, error) {
	rinfo := &webvcs.RunInfo{}
	runinfo.DeepCopyInto(rinfo)
	rinfo.EventType = ""
	rinfo.TriggerTarget = ""
	rinfo.URL = rinfo.Event.(*github.IssueCommentEvent).Issue.GetPullRequestLinks().GetHTMLURL()
	comments, err := cs.GithubClient.GetStringPullRequestComment(ctx, rinfo, okToTestCommentRegexp)
	if err != nil {
		return false, err
	}

	for _, comment := range comments {
		rinfo.Sender = comment.User.GetLogin()
		allowed, err := aclCheck(ctx, cs, rinfo)
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
	}
	return false, nil
}

// aclCheck check if we are allowed to run the pipeline on that PR
func aclCheck(ctx context.Context, cs *cli.Clients, runinfo *webvcs.RunInfo) (bool, error) {
	if runinfo.Owner == runinfo.Sender {
		return true, nil
	}

	// If the user who has submitted the pr is a owner on the repo then allows
	// the CI to be run.
	isUserMemberRepo, err := cs.GithubClient.CheckSenderOrgMembership(ctx, runinfo)
	if err != nil {
		return false, err
	}

	if isUserMemberRepo {
		return true, nil
	}

	// If we have a prow OWNERS file in the defaultBranch (ie: master) then
	// parse it in approvers and reviewers field and check if sender is in there.
	ownerFile, err := cs.GithubClient.GetFileFromDefaultBranch(ctx, "OWNERS", runinfo)

	// Don't error out if the OWNERS file cannot be found
	if err != nil && !strings.Contains(err.Error(), "cannot find") {
		return false, err
	} else if ownerFile != "" {
		var ownerConfig OwnersConfig
		err := yaml.Unmarshal([]byte(ownerFile), &ownerConfig)
		if err != nil {
			return false, err
		}
		for _, owner := range append(ownerConfig.Approvers, ownerConfig.Reviewers...) {
			if owner == runinfo.Sender {
				return true, nil
			}
		}
	}

	if runinfo.TriggerTarget == "ok-to-test-comment" {
		allowed, err := allowedOkToTestFromAnOwner(ctx, cs, runinfo)
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
	}

	return false, nil
}
