package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dantoml/branchbot/internal/version"
	"github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const (
	// http://patorjk.com/software/taag/#p=display&f=Small&t=BranchBot
	INFO = `  ___                  _    ___      _
 | _ )_ _ __ _ _ _  __| |_ | _ ) ___| |_
 | _ \ '_/ _` + "`" + ` | ' \/ _| ' \| _ \/ _ \  _|
 |___/_| \__,_|_||_\__|_||_|___/\___/\__|

 Version: %s-%s

 A GitHub bot to automatically delete branches on your repos when PRs are merged.

`
)

var (
	token           string
	interval        int
	rawRepos        string
	verbose         bool
	onlyOwnedBySelf bool

	versionCmd bool

	repos    []string
	username string
)

func init() {
	flag.StringVar(&token, "token", os.Getenv("GITHUB_TOKEN"), "GitHub API Token")
	flag.IntVar(&interval, "interval", 30, "Check interval in seconds")
	flag.StringVar(&rawRepos, "repos", os.Getenv("GITHUB_REPOS"), "GitHub Repos (e.g dantoml/branchbot,cocoapods/cocoapods)")
	flag.BoolVar(&onlyOwnedBySelf, "self-only", true, "Only delete branches owned by self")

	flag.BoolVar(&verbose, "verbose", false, "Enable debug logging")
	flag.BoolVar(&versionCmd, "version", false, "Print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, INFO, version.UserVersion, version.BuildNumber)
		flag.PrintDefaults()
	}

	flag.Parse()

	if versionCmd == true {
		fmt.Printf("branchbot version: %s-%s\n", version.UserVersion, version.BuildNumber)
		os.Exit(0)
	}

	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if token == "" {
		fmt.Fprintf(os.Stderr, "GitHub token cannot be empty.\n\n")
		flag.Usage()
		fmt.Fprintf(os.Stderr, "\n")
		os.Exit(1)
	}

	if rawRepos == "" {
		fmt.Fprintf(os.Stderr, "repos cannot be empty. \n\n")
		flag.Usage()
		fmt.Fprintf(os.Stderr, "\n")
		os.Exit(1)
	} else {
		repos = strings.Split(rawRepos, ",")
	}

	logrus.SetFormatter(&logrus.TextFormatter{})
}

func main() {
	logrus.Info("Starting branchbot")

	var ticker *time.Ticker
	// On ^C/SIGTERM handle exit.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		for sig := range c {
			ticker.Stop()
			logrus.Infof("Received %s, exiting.", sig.String())
			os.Exit(0)
		}
	}()

	ctx := context.Background()

	client, err := newGitHubClient(ctx, token)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Debug("Starting ticker")
	ticker = time.NewTicker(time.Duration(interval) * time.Second)
	for range ticker.C {
		page := 1
		perPage := 30
		for _, repo := range repos {
			logger := logrus.WithFields(logrus.Fields{"repo": repo})
			logger.Info("Starting")
			if err := handlePullRequests(ctx, logger, client, repo, page, perPage); err != nil {
				logger.Warn(err)
			}
		}
	}
}

func newGitHubClient(ctx context.Context, token string) (*github.Client, error) {
	// Create the http client.
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	// Create the github client.
	client := github.NewClient(tc)

	// Get the authenticated user to validate that the token works.
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, err
	}
	username = *user.Login

	logrus.WithFields(logrus.Fields{"Username": username}).Info("Authenticated.")

	return client, nil
}

func handlePullRequests(ctx context.Context, logger *logrus.Entry, client *github.Client, repoIdentifier string, page, perPage int) error {
	components := strings.Split(repoIdentifier, "/")
	requestOptions := &github.PullRequestListOptions{
		State: "closed",
		Sort:  "updated",
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: perPage,
		},
	}
	pullRequests, _, err := client.PullRequests.List(ctx, components[0], components[1], requestOptions)

	if err != nil {
		return err
	}

	logger.Debug("Fetched recent pull requests")

	for _, request := range pullRequests {
		err = handlePullRequest(ctx, logger, client, request)
		if err != nil {
			return err
		}
	}

	return nil
}

func handlePullRequest(ctx context.Context, logger *logrus.Entry, client *github.Client, pr *github.PullRequest) error {
	if *pr.State != "closed" {
		return nil
	}

	branch := *pr.Head.Ref
	if pr.Head.Repo == nil {
		return nil
	}

	if pr.Head.Repo.Owner == nil {
		return nil
	}

	owner := *pr.Head.Repo.Owner.Login
	if onlyOwnedBySelf && owner == username {
		logger.Debug("Skipping branch because it is not owned by self")
		return nil
	}

	if branch != *pr.Head.Repo.DefaultBranch {
		_, err := client.Git.DeleteRef(ctx, *pr.Head.Repo.Owner.Login, *pr.Head.Repo.Name, strings.Replace("heads/"+*pr.Head.Ref, "#", "%23", -1))
		// 422 is the error code for when the branch does not exist.
		if err != nil && !strings.Contains(err.Error(), " 422 ") {
			return err
		}

		logger.WithFields(logrus.Fields{"branch": branch, "owner": owner, "repo": *pr.Head.Repo.Name}).Infof("Branch has been deleted.")
	}

	return nil
}
