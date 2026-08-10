package main

import (
	"fmt"

	"github.com/jallum/beadwork/internal/config"

	"github.com/jallum/beadwork/internal/issue"
	"github.com/jallum/beadwork/internal/md"
)

type ReviewArgs struct {
	ID   string
	JSON bool
}

func parseReviewArgs(raw []string) (ReviewArgs, error) {
	a, err := ParseArgs(raw, nil, []string{"--json"})
	if err != nil {
		return ReviewArgs{}, err
	}
	id := a.PosFirst()
	if id == "" {
		return ReviewArgs{}, fmt.Errorf("usage: bw review <id>")
	}
	return ReviewArgs{ID: id, JSON: a.JSON()}, nil
}

func cmdReview(store *issue.Store, args []string, w Writer, _ *config.Config) (*config.Config, error) {
	ra, err := parseReviewArgs(args)
	if err != nil {
		return nil, err
	}

	var iss *issue.Issue
	err = commitWithRetry(store, commitMaxRetries, func() (string, error) {
		var rerr error
		iss, rerr = store.Review(ra.ID)
		if rerr != nil {
			return "", rerr
		}
		return fmt.Sprintf("review %s", iss.ID), nil
	})
	if err != nil {
		return nil, err
	}

	if ra.JSON {
		fprintJSON(w, iss)
	} else {
		fmt.Fprintf(w, "in review {id:%s}: %s\n", iss.ID, md.Escape(iss.Title))
		fmt.Fprintf(w, "Next: `bw close %s` on merge, or `bw update %s --status in_progress` if review sends it back.\n", iss.ID, iss.ID)
	}
	return nil, nil
}
