// Command makeutility is the CLI engineers run day-to-day: sync clones and
// updates every team repo concurrently, and status reports each repo's
// local state. See proposal.md at the repository root for the full design.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/joshuakaki/makeutility/internal/gitops"
)

const (
	defaultMaxConcurrent = 8
	defaultReposFile     = "repos.yaml"
	ansiRed              = "\x1b[31m"
	ansiYellow           = "\x1b[33m"
	ansiGreen            = "\x1b[32m"
	ansiReset            = "\x1b[0m"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var err error
	switch os.Args[1] {
	case "sync":
		err = runSync(ctx)
	case "status":
		err = runStatus(ctx)
	default:
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "makeutility:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: makeutility <sync|status>")
}

// loadSpecs tries makeutility-api first, falling back to the local
// repos.yaml so the CLI keeps working if the API is unreachable.
func loadSpecs(ctx context.Context) ([]gitops.RepoSpec, error) {
	apiURL := os.Getenv("MAKEUTILITY_API_URL")
	apiKey := os.Getenv("MAKEUTILITY_API_KEY")
	if apiURL != "" && apiKey != "" {
		specs, err := gitops.FetchReposFromAPI(ctx, apiURL, apiKey)
		if err == nil {
			return specs, nil
		}
		fmt.Fprintln(os.Stderr, "makeutility: could not reach makeutility-api, falling back to", defaultReposFile, "-", err)
	}
	return gitops.LoadReposYAML(defaultReposFile)
}

func workspaceDir() string {
	if dir := os.Getenv("MAKEUTILITY_WORKSPACE"); dir != "" {
		return dir
	}
	return "."
}

func runSync(ctx context.Context) error {
	specs, err := loadSpecs(ctx)
	if err != nil {
		return err
	}

	states := gitops.Sync(ctx, workspaceDir(), specs, defaultMaxConcurrent)

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "REPO\tRESULT")
	failures := 0
	for _, s := range states {
		switch {
		case s.Err != nil:
			failures++
			_, _ = fmt.Fprintf(tw, "%s\t%serror: %v%s\n", s.Name, ansiRed, s.Err, ansiReset)
		case s.Cloned:
			_, _ = fmt.Fprintf(tw, "%s\t%scloned%s\n", s.Name, ansiGreen, ansiReset)
		default:
			_, _ = fmt.Fprintf(tw, "%s\t%sup to date%s\n", s.Name, ansiGreen, ansiReset)
		}
	}
	_ = tw.Flush()

	if failures > 0 {
		return fmt.Errorf("%d repo(s) failed to sync", failures)
	}
	return nil
}

func runStatus(ctx context.Context) error {
	specs, err := loadSpecs(ctx)
	if err != nil {
		return err
	}

	states := gitops.Status(ctx, workspaceDir(), specs, defaultMaxConcurrent)

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "REPO\tBRANCH\tAHEAD\tBEHIND\tSTATE")
	for _, s := range states {
		if s.Err != nil {
			_, _ = fmt.Fprintf(tw, "%s\t-\t-\t-\t%serror: %v%s\n", s.Name, ansiRed, s.Err, ansiReset)
			continue
		}
		state := ansiGreen + "clean" + ansiReset
		if s.Dirty {
			state = ansiYellow + "dirty" + ansiReset
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\n", s.Name, s.Branch, s.Ahead, s.Behind, state)
	}
	_ = tw.Flush()

	return nil
}
