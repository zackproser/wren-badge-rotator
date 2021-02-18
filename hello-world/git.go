package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

const (
	REPO_URL = "https://github.com/zackproser/zackproser.git"
)

func getGithubClient() (*github.Client, error) {
	if os.Getenv("GITHUB_OAUTH_TOKEN") == "" {
		return nil, errors.New("You must set the GITHUB_OAUTH_TOKEN env var to a valid Github personal access token")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_OAUTH_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	return client, nil
}

func cloneRepo() (string, *git.Repository, error) {

	repositoryDir, tmpDirErr := ioutil.TempDir("", "wren-badge-rotator")
	if tmpDirErr != nil {
		return "", nil, tmpDirErr
	}

	localRepository, err := git.PlainClone(repositoryDir, false, &git.CloneOptions{
		URL:      REPO_URL,
		Progress: os.Stdout,
	})

	if err != nil {
		return "", nil, err
	}

	return repositoryDir, localRepository, nil
}

// getLocalRepoHeadRef looks up the HEAD reference of the locally cloned git repository, which is required by
// downstream operations such as branching
func getLocalRepoHeadRef(localRepository *git.Repository) (*plumbing.Reference, error) {
	ref, headErr := localRepository.Head()
	if headErr != nil {
		return nil, headErr
	}
	return ref, nil
}

// getLocalWorkTree looks up the working tree of the locally cloned repository and returns it if possible, or an error
func getLocalWorkTree(repositoryDir string, localRepository *git.Repository) (*git.Worktree, error) {
	worktree, worktreeErr := localRepository.Worktree()

	if worktreeErr != nil {
		return nil, worktreeErr
	}
	return worktree, nil
}

// checkoutLocalBranch creates a local branch specific to this tool in the locally checked out copy of the repo in the /tmp folder
func checkoutLocalBranch(ref *plumbing.Reference, worktree *git.Worktree, localRepository *git.Repository) (plumbing.ReferenceName, error) {

	t := time.Now()
	month := t.Month()

	branchNameWithMonth := fmt.Sprintf("update-wren-badge-%s", month)

	// BranchName is a global variable that is set in cmd/root.go. It is override-able by the operator via the --branch-name or -b flag. It defaults to "multi-repo-script-runner"
	branchName := plumbing.NewBranchReferenceName(branchNameWithMonth)
	// Create a branch specific to the multi repo script runner
	co := &git.CheckoutOptions{
		Hash:   ref.Hash(),
		Branch: branchName,
		Create: true,
	}

	// Attempt to checkout the new tool-specific branch on which all scripts will be executed
	checkoutErr := worktree.Checkout(co)

	if checkoutErr != nil {
		return branchName, checkoutErr
	}

	return branchName, nil
}

// commitLocalChanges will create a commit using the supplied or default commit message and will add any untracked, deleted
// or modified files that resulted from script execution
func commitLocalChanges(worktree *git.Worktree, localRepository *git.Repository) error {

	t := time.Now()
	month := t.Month()

	commitMessage := fmt.Sprintf("Update Project Wren Badge with monthly stats for %s", month)

	// With all our untracked files staged, we can now create a commit, passing the All
	// option when configuring our commit option so that all modified and deleted files
	// will have their changes committed
	commitOps := &git.CommitOptions{
		All: true,
	}

	_, commitErr := worktree.Commit(commitMessage, commitOps)

	if commitErr != nil {
		return commitErr
	}

	return nil
}

// pushLocalBranch pushes the branch in the local clone of the /tmp/ directory repository to the Github remote origin
// so that a pull request can be opened against it via the Github API
func pushLocalBranch(localRepository *git.Repository) error {
	// Push the changes to the remote repo
	po := &git.PushOptions{
		RemoteName: "origin",
	}
	pushErr := localRepository.Push(po)

	if pushErr != nil {
		return pushErr
	}

	return nil
}

// Attempt to open a pull request via the Github API, of the supplied branch specific to this tool, against the main
// branch for the remote origin
func openPullRequest(GithubClient *github.Client, branch string) error {

	t := time.Now()
	month := t.Month()

	pullRequestTitle := fmt.Sprintf("Update Project Wren Badge for %s", month)
	pullRequestDescription := fmt.Sprintf("Swap in the latest badge with the stats for %s", month)

	repoOwner := "zackproser"
	repoName := "zackproser"

	// Configure pull request options that the Github client accepts when making calls to open new pull requests
	newPR := &github.NewPullRequest{
		Title:               github.String(pullRequestTitle),
		Head:                github.String(branch),
		Base:                github.String("master"),
		Body:                github.String(pullRequestDescription),
		MaintainerCanModify: github.Bool(true),
	}

	// Make a pull request via the Github API
	pr, _, err := GithubClient.PullRequests.Create(context.Background(), repoOwner, repoName, newPR)

	if err != nil {
		return err
	}

	fmt.Printf("Successfully opened Pull Request: %s", pr.GetHTMLURL())

	return nil
}

func updateBadgeContents(repositoryDir string) error {

	badgePath := path.Join(repositoryDir, "img")

	targetBadgeFile, err := os.Create(badgePath)

	if err != nil {
		return err
	}

	defer targetBadgeFile.Close()

	updatedBadgeFile, err := os.Open(EXTRACTED_BADGE_IMAGE_LOCAL_PATH)
	if err != nil {
		return err
	}

	_, err = io.Copy(targetBadgeFile, updatedBadgeFile)

	if err != nil {
		return err
	}

	return nil
}

func updateBadgeImage() error {

	repositoryDir, localRepository, cloneErr := cloneRepo()

	if cloneErr != nil {
		return cloneErr
	}

	ref, headRefErr := getLocalRepoHeadRef(localRepository)

	if headRefErr != nil {
		return headRefErr
	}

	worktree, worktreeErr := getLocalWorkTree(repositoryDir, localRepository)

	if worktreeErr != nil {
		return worktreeErr
	}

	branchName, branchErr := checkoutLocalBranch(ref, worktree, localRepository)

	if branchErr != nil {
		return branchErr
	}

	updateErr := updateBadgeContents(repositoryDir)

	if updateErr != nil {
		return updateErr
	}

	commitErr := commitLocalChanges(worktree, localRepository)
	if commitErr != nil {
		return commitErr
	}

	pushErr := pushLocalBranch(localRepository)

	if pushErr != nil {
		return pushErr
	}

	githubClient, clientErr := getGithubClient()

	if clientErr != nil {
		return clientErr
	}

	openPRErr := openPullRequest(githubClient, branchName.String())
	if openPRErr != nil {
		return openPRErr
	}

	return nil
}
