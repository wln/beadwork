package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jallum/beadwork/internal/issue"
	"github.com/jallum/beadwork/internal/testutil"
)

func TestCmdReviewBasic(t *testing.T) {
	env := testutil.NewEnv(t)
	defer env.Cleanup()

	iss, _ := env.Store.Create("Review me", issue.CreateOpts{})
	env.Repo.Commit("create " + iss.ID)
	env.Store.Start(iss.ID, "alice")
	env.Repo.Commit("start " + iss.ID)

	var buf bytes.Buffer
	_, err := cmdReview(env.Store, []string{iss.ID}, PlainWriter(&buf), nil)
	if err != nil {
		t.Fatalf("cmdReview: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "in review") {
		t.Errorf("output missing 'in review': %q", out)
	}
	if !strings.Contains(out, "Review me") {
		t.Errorf("output missing title: %q", out)
	}

	got, _ := env.Store.Get(iss.ID)
	if got.Status != "in_review" {
		t.Errorf("status = %q, want in_review", got.Status)
	}
	if got.Assignee != "alice" {
		t.Errorf("assignee = %q, want alice (kept)", got.Assignee)
	}
}

func TestCmdReviewJSON(t *testing.T) {
	env := testutil.NewEnv(t)
	defer env.Cleanup()

	iss, _ := env.Store.Create("Review JSON", issue.CreateOpts{})
	env.Repo.Commit("create " + iss.ID)
	env.Store.Start(iss.ID, "alice")
	env.Repo.Commit("start " + iss.ID)

	var buf bytes.Buffer
	_, err := cmdReview(env.Store, []string{iss.ID, "--json"}, PlainWriter(&buf), nil)
	if err != nil {
		t.Fatalf("cmdReview --json: %v", err)
	}
	var got issue.Issue
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %q", err, buf.String())
	}
	if got.Status != "in_review" {
		t.Errorf("status = %q, want in_review", got.Status)
	}
}

func TestCmdReviewNotInProgress(t *testing.T) {
	env := testutil.NewEnv(t)
	defer env.Cleanup()

	iss, _ := env.Store.Create("Still open", issue.CreateOpts{})
	env.Repo.Commit("create " + iss.ID)

	var buf bytes.Buffer
	_, err := cmdReview(env.Store, []string{iss.ID}, PlainWriter(&buf), nil)
	if err == nil {
		t.Fatal("expected error reviewing open issue")
	}
}

func TestCmdUpdateRejectsUnknownStatus(t *testing.T) {
	env := testutil.NewEnv(t)
	defer env.Cleanup()

	iss, _ := env.Store.Create("Typo target", issue.CreateOpts{})
	env.Repo.Commit("create " + iss.ID)

	var buf bytes.Buffer
	_, err := cmdUpdate(env.Store, []string{iss.ID, "--status", "in-review"}, PlainWriter(&buf), nil)
	if err == nil {
		t.Fatal("expected error for unknown status")
	}
	if !strings.Contains(err.Error(), "unknown status") {
		t.Errorf("error = %q, want mention of unknown status", err.Error())
	}
}

func TestCmdListDefaultIncludesInReview(t *testing.T) {
	env := testutil.NewEnv(t)
	defer env.Cleanup()

	iss, _ := env.Store.Create("In review row", issue.CreateOpts{})
	env.Repo.Commit("create " + iss.ID)
	env.Store.Start(iss.ID, "alice")
	env.Store.Review(iss.ID)
	env.Repo.Commit("review " + iss.ID)

	var buf bytes.Buffer
	_, err := cmdList(env.Store, nil, PlainWriter(&buf), nil)
	if err != nil {
		t.Fatalf("cmdList: %v", err)
	}
	if !strings.Contains(buf.String(), "In review row") {
		t.Errorf("default list missing in_review issue: %q", buf.String())
	}
}
