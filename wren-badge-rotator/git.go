package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

// getGithubClient uses the Github personal access token defined as an environment variable to create a new Github API client
// This client will be used to make the API call to Github to create the Pull Request updating the badge
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

// cloneRepo uses the go-git library to clone my Github profile repository to a newly created /tmp/directory
// This way the badge image can be updated and committed in place, and pushed during execution, and it's fine for
// everything else to be discarded following the lambda execution, since this function will tend to be run once per month on average
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

	// Create a branch name that contains the Month so that it's easier to scan and understand
	branchNameWithMonth := fmt.Sprintf("update-wren-badge-%s", month)

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

// commitLocalChanges will commit the modified badge image to the local checkout of the repo so that it can be pushed to the remote origin next
func commitLocalChanges(worktree *git.Worktree, localRepository *git.Repository, repositoryDir string) error {

	t := time.Now()
	month := t.Month()

	// Create a commit message that contains the current month for easier scanning
	commitMessage := fmt.Sprintf("Update Project Wren Badge with monthly stats for %s", month)

	// We can now create a commit, passing the All
	// option when configuring our commit option so that all modified and deleted files
	// will have their changes committed
	commitOps := &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  "Zack Proser",
			Email: "zackproser@gmail.com",
			When:  time.Now(),
		},
	}

	_, commitErr := worktree.Commit(commitMessage, commitOps)

	if commitErr != nil {
		return commitErr
	}

	return nil
}

// pushLocalBranch pushes the branch in the local clone of the /tmp/ directory repository to the Github remote origin
// so that a pull request can be opened against it via the Github API. Note this step requires http.BasicAuth to perform
// so I log identify myself to Github via my username and my Github personal access token as my password
func pushLocalBranch(localRepository *git.Repository) error {
	// Push the changes to the remote repo
	po := &git.PushOptions{
		RemoteName: "origin",
		Auth: &http.BasicAuth{
			Username: os.Getenv("REPO_OWNER"),
			Password: os.Getenv("GITHUB_OAUTH_TOKEN"),
		},
	}
	pushErr := localRepository.Push(po)

	if pushErr != nil {
		fmt.Println("Error pushing local branch to remote")
		return pushErr
	}

	return nil
}

// Attempt to open a pull request via the Github API, of the branch containing the badge changes against the master branch
func openPullRequest(GithubClient *github.Client, branch string) error {

	t := time.Now()
	month := t.Month()

	pullRequestTitle := fmt.Sprintf("Update Project Wren Badge for %s", month)
	pullRequestDescription := fmt.Sprintf("Swap in the latest badge with the stats for %s", month)

	repoName := os.Getenv("REPO_OWNER")

	// Configure pull request options that the Github client accepts when making calls to open new pull requests
	newPR := &github.NewPullRequest{
		Title:               github.String(pullRequestTitle),
		Head:                github.String(branch),
		Base:                github.String("master"),
		Body:                github.String(pullRequestDescription),
		MaintainerCanModify: github.Bool(true),
	}

	// Make a pull request via the Github API
	pr, _, err := GithubClient.PullRequests.Create(context.Background(), repoName, repoName, newPR)

	if err != nil {
		return err
	}

	fmt.Printf("Successfully opened Pull Request: %s", pr.GetHTMLURL())

	return nil
}

// updateBadgeContents will intentionally overwrite the existing /img/carbon-wren.png badge that exists in the locally checked out repository
// by copying over it the local version of the updated badge image
func updateBadgeContents(repositoryDir string) error {

	badgePath := path.Join(repositoryDir, "img", "carbon-wren.png")

	// Overwrite the existing local repo's copy of the previous badge with the freshly extracted and updated badge
	cmd := exec.Command("cp", EXTRACTED_BADGE_IMAGE_LOCAL_PATH, badgePath)
	stdout, err := cmd.Output()

	if err != nil {
		return err
	}

	fmt.Printf("cp updated badge into place output: %s\n", string(stdout))

	return nil
}

// updateBadgeImage wraps all the operations that need to occur in order to update the badge image on my Github profile:
// 1. Clone the zackproser/zackproser repository to a local /tmp directory
// 2. Get the HEAD ref from that repository for use in branching
// 3. Get the local worktree of that repository for use in commiting changes
// 4. Checkout a new local branch specific to the month the update is being run in
// 5. Overwrite the badge image contents that are currently in the Github repository's img directory with the contents
// of the badge that have now been scraped from wren and then processed into an image via the HCTI API and then
// stored under the /extracted prefix within the S3 bucket
// 6. Commit this file change, using my own signature
// 7. Push the local branch to the remote origin, using my Github personal access token and HTTP basic auth as transport.Auth scheme
// 8. Using my Github personal access token, obtain a Github API client and make a call to create a Pull Request
func updateBadgeImage() error {

	repositoryDir, localRepository, cloneErr := cloneRepo()

	if cloneErr != nil {
		return cloneErr
	}

	fmt.Printf("Local repository cloned to: %s\n", repositoryDir)

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

	commitErr := commitLocalChanges(worktree, localRepository, repositoryDir)
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
