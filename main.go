package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	version = "2.0.0"
)

// Exit codes
const (
	ExitSuccess        = 0
	ExitConflictsFound = 1 // Conflicts detected, user action needed
	ExitError          = 2 // Actual error occurred
)

// Sentinel error for conflicts (not a real error, just signals user action needed)
var errConflicts = errors.New("conflicts")

// State directory inside .git
const anticipateDir = "anticipate"

func main() {
	var rootCmd = &cobra.Command{
		Use:   "git-anticipate [target-branch]",
		Short: "Preemptively resolve merge conflicts in-place",
		Long: `git-anticipate helps you resolve merge conflicts before they happen.

It performs a trial merge in your working directory, lets you resolve
conflicts, and then applies your resolution as a regular commit.

Usage:
  git anticipate <target-branch>    Start anticipating conflicts with target branch
  git anticipate --continue         Apply resolved conflicts as a commit
  git anticipate --abort            Abort and restore original state
  git anticipate --status           Show current anticipate status`,
		Args:          cobra.MaximumNArgs(1),
		RunE:          runAnticipate,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var continueFlag bool
	var abortFlag bool
	var statusFlag bool
	var noVerifyFlag bool

	rootCmd.Flags().BoolVar(&continueFlag, "continue", false, "Continue after resolving conflicts")
	rootCmd.Flags().BoolVar(&abortFlag, "abort", false, "Abort and restore original state")
	rootCmd.Flags().BoolVar(&statusFlag, "status", false, "Show current anticipate status")
	rootCmd.Flags().BoolVar(&noVerifyFlag, "no-verify", false, "Skip pre-commit hooks when committing")
	rootCmd.Version = version

	if err := rootCmd.Execute(); err != nil {
		if err == errConflicts {
			os.Exit(ExitConflictsFound)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(ExitError)
	}
}

func runAnticipate(cmd *cobra.Command, args []string) error {
	continueFlag, _ := cmd.Flags().GetBool("continue")
	abortFlag, _ := cmd.Flags().GetBool("abort")
	statusFlag, _ := cmd.Flags().GetBool("status")

	// Validate we're in a git repo
	if err := validateRepo(); err != nil {
		return err
	}

	// Get git dir path
	gitDir, err := getGitDir()
	if err != nil {
		return fmt.Errorf("failed to get git directory: %w", err)
	}
	stateDir := filepath.Join(gitDir, anticipateDir)

	// Handle flags
	if statusFlag {
		return showStatus(stateDir)
	}

	if abortFlag {
		return abortAnticipate(stateDir)
	}

	if continueFlag {
		noVerify, _ := cmd.Flags().GetBool("no-verify")
		return continueAnticipate(stateDir, noVerify)
	}

	// Start new anticipate
	if len(args) == 0 {
		// Check if anticipate is in progress
		if isAnticipateInProgress(stateDir) {
			return showStatus(stateDir)
		}
		return fmt.Errorf("usage: git anticipate <target-branch>\n       git anticipate --continue | --abort | --status")
	}

	return startAnticipate(stateDir, args[0])
}

// startAnticipate begins a new anticipate session
func startAnticipate(stateDir, targetBranch string) error {
	fmt.Printf("🚀 git-anticipate: Preemptive conflict resolution\n")
	fmt.Printf("Target branch: %s\n\n", targetBranch)

	// Check if anticipate already in progress
	if isAnticipateInProgress(stateDir) {
		return fmt.Errorf("anticipate already in progress\nUse 'git anticipate --continue' or 'git anticipate --abort'")
	}

	// Check for uncommitted changes
	if hasUncommittedChanges() {
		return fmt.Errorf("you have uncommitted changes\nPlease commit or stash them before running git anticipate")
	}

	// Get current branch
	currentBranch, err := getCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	fmt.Printf("Current branch: %s\n", currentBranch)

	// Validate target branch exists
	if err := validateBranchExists(targetBranch); err != nil {
		return fmt.Errorf("target branch '%s' does not exist", targetBranch)
	}

	// Get current HEAD
	origHead, err := getRevisionSHA("HEAD")
	if err != nil {
		return fmt.Errorf("failed to get current HEAD: %w", err)
	}

	// Get target branch SHA
	targetSHA, err := getRevisionSHA(targetBranch)
	if err != nil {
		return fmt.Errorf("failed to get target branch SHA: %w", err)
	}

	// Get merge base
	baseSHA, err := getMergeBase(targetBranch, currentBranch)
	if err != nil {
		return fmt.Errorf("failed to get merge base: %w", err)
	}
	fmt.Printf("Merge base: %s\n\n", truncateSHA(baseSHA))

	// Save state
	if err := saveState(stateDir, targetBranch, origHead, targetSHA, currentBranch); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Attempt merge
	fmt.Printf("✔ Attempting merge with %s...\n", targetBranch)
	mergeResult, mergeErr := performMerge(targetBranch)

	switch mergeResult {
	case MergeConflict:
		conflictFiles := getConflictingFiles()
		fmt.Printf("\n⚠️  Conflicts detected!\n\n")

		if len(conflictFiles) > 0 {
			fmt.Printf("Conflicting files (%d):\n", len(conflictFiles))
			for _, file := range conflictFiles {
				fmt.Printf("    ❌ %s\n", file)
			}
			fmt.Printf("\n")
		}

		fmt.Printf("Resolve conflicts in your working directory, then:\n")
		fmt.Printf("  git add <resolved-files>\n")
		fmt.Printf("  git anticipate --continue\n\n")
		fmt.Printf("Or to abort:\n")
		fmt.Printf("  git anticipate --abort\n")

		return errConflicts

	case MergeError:
		// Clean up state on error
		removeState(stateDir)
		return mergeErr

	case MergeClean:
		// Check if there are any changes
		if !hasStagedOrUnstagedChanges() {
			fmt.Printf("✨ No conflicts and no changes - branches are already compatible!\n")
			removeState(stateDir)
			return nil
		}

		fmt.Printf("✔ No conflicts detected!\n")
		fmt.Printf("\nChanges from %s are staged. To apply as a preparation commit:\n", targetBranch)
		fmt.Printf("  git anticipate --continue\n\n")
		fmt.Printf("Or to abort:\n")
		fmt.Printf("  git anticipate --abort\n")

		return nil
	}

	return nil
}

// continueAnticipate applies the resolution and creates a commit
func continueAnticipate(stateDir string, noVerify bool) error {
	if !isAnticipateInProgress(stateDir) {
		return fmt.Errorf("no anticipate in progress")
	}

	// Check for unresolved conflicts
	if hasUnmergedFiles() {
		conflictFiles := getConflictingFiles()
		fmt.Printf("⚠️  Unresolved conflicts remain:\n")
		for _, file := range conflictFiles {
			fmt.Printf("    ❌ %s\n", file)
		}
		fmt.Printf("\nResolve conflicts and run 'git add', then 'git anticipate --continue'\n")
		return errConflicts
	}

	// Read state
	targetBranch, err := readStateFile(stateDir, "target")
	if err != nil {
		return fmt.Errorf("failed to read target branch: %w", err)
	}

	targetSHA, err := readStateFile(stateDir, "target_sha")
	if err != nil {
		return fmt.Errorf("failed to read target SHA: %w", err)
	}

	currentBranch, err := readStateFile(stateDir, "current_branch")
	if err != nil {
		return fmt.Errorf("failed to read current branch: %w", err)
	}

	origHead, err := readStateFile(stateDir, "orig_head")
	if err != nil {
		return fmt.Errorf("failed to read original HEAD: %w", err)
	}

	fmt.Printf("🚀 git-anticipate: Applying resolution\n\n")

	// Stage all changes (in case user only did git add for some files)
	stageCmd := exec.Command("git", "add", "-u")
	stageCmd.Run()

	// Get list of files that have changes (staged)
	changedFilesCmd := exec.Command("git", "diff", "--cached", "--name-only")
	changedFilesOutput, err := changedFilesCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get changed files: %w", err)
	}

	changedFiles := []string{}
	for _, f := range strings.Split(strings.TrimSpace(string(changedFilesOutput)), "\n") {
		if f != "" {
			changedFiles = append(changedFiles, f)
		}
	}

	// Check if there's anything to commit
	if len(changedFiles) == 0 {
		fmt.Printf("✨ No changes to commit - branches are compatible!\n")
		abortMerge()
		removeState(stateDir)
		return nil
	}

	// Get list of deleted files separately
	deletedFilesCmd := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=D")
	deletedFilesOutput, _ := deletedFilesCmd.Output()
	deletedFiles := make(map[string]bool)
	for _, f := range strings.Split(strings.TrimSpace(string(deletedFilesOutput)), "\n") {
		if f != "" {
			deletedFiles[f] = true
		}
	}

	fmt.Printf("✔ Extracting resolution (%d files)...\n", len(changedFiles))

	// Save the content of all changed files BEFORE aborting merge
	// (skip deleted files - we'll handle them separately)
	fileContents := make(map[string][]byte)
	for _, file := range changedFiles {
		if deletedFiles[file] {
			continue // Skip deleted files
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read resolved file %s: %w", file, err)
		}
		fileContents[file] = content
	}

	// Abort the merge
	abortMerge()

	// Reset to original HEAD to ensure clean state
	resetCmd := exec.Command("git", "reset", "--hard", origHead)
	if err := resetCmd.Run(); err != nil {
		return fmt.Errorf("failed to reset to original state: %w", err)
	}

	// Write back the resolved file contents
	fmt.Printf("✔ Applying resolution to %s...\n", currentBranch)
	for file, content := range fileContents {
		// Ensure directory exists
		dir := filepath.Dir(file)
		if dir != "." {
			os.MkdirAll(dir, 0755)
		}
		if err := os.WriteFile(file, content, 0644); err != nil {
			return fmt.Errorf("failed to write resolved file %s: %w", file, err)
		}
	}

	// Handle deleted files - remove them
	for file := range deletedFiles {
		os.Remove(file) // Ignore errors - file might not exist
	}

	// Stage all the changed files
	for _, file := range changedFiles {
		if deletedFiles[file] {
			// For deleted files, use git rm
			rmCmd := exec.Command("git", "rm", "--cached", "--ignore-unmatch", file)
			rmCmd.Run()
		} else {
			addCmd := exec.Command("git", "add", file)
			if err := addCmd.Run(); err != nil {
				return fmt.Errorf("failed to stage file %s: %w", file, err)
			}
		}
	}

	// Create commit
	commitMsg := fmt.Sprintf("Preemptive conflict resolution vs %s@%s", targetBranch, truncateSHA(targetSHA))
	fmt.Printf("✔ Creating commit...\n")

	commitArgs := []string{"commit", "-m", commitMsg}
	if noVerify {
		commitArgs = append(commitArgs, "--no-verify")
	}
	commitCmd := exec.Command("git", commitArgs...)
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("failed to create commit: %w\n\nTip: If pre-commit hooks are failing, you can:\n  1. Fix the issues and run 'git anticipate --continue' again\n  2. Or run 'git anticipate --continue --no-verify' to skip hooks", err)
	}

	// Clean up state
	removeState(stateDir)

	fmt.Printf("\n✨ Success! Resolution committed to %s\n", currentBranch)
	fmt.Printf("Your branch is now prepared for merging into %s\n", targetBranch)

	return nil
}

// abortAnticipate aborts the current anticipate session
func abortAnticipate(stateDir string) error {
	if !isAnticipateInProgress(stateDir) {
		return fmt.Errorf("no anticipate in progress")
	}

	fmt.Printf("🚀 git-anticipate: Aborting\n\n")

	// Abort any merge in progress
	abortMerge()

	// Read original HEAD
	origHead, err := readStateFile(stateDir, "orig_head")
	if err != nil {
		return fmt.Errorf("failed to read original HEAD: %w", err)
	}

	// Reset to original HEAD
	fmt.Printf("✔ Restoring original state...\n")
	resetCmd := exec.Command("git", "reset", "--hard", origHead)
	if err := resetCmd.Run(); err != nil {
		return fmt.Errorf("failed to reset to original state: %w", err)
	}

	// Clean up state
	removeState(stateDir)

	fmt.Printf("✔ Anticipate aborted. Restored to original state.\n")
	return nil
}

// showStatus shows the current anticipate status
func showStatus(stateDir string) error {
	if !isAnticipateInProgress(stateDir) {
		fmt.Printf("No anticipate in progress.\n\n")
		fmt.Printf("Usage: git anticipate <target-branch>\n")
		return nil
	}

	targetBranch, _ := readStateFile(stateDir, "target")
	currentBranch, _ := readStateFile(stateDir, "current_branch")
	origHead, _ := readStateFile(stateDir, "orig_head")

	fmt.Printf("🚀 git-anticipate: In Progress\n\n")
	fmt.Printf("Current branch:  %s\n", currentBranch)
	fmt.Printf("Target branch:   %s\n", targetBranch)
	fmt.Printf("Original HEAD:   %s\n", truncateSHA(origHead))
	fmt.Printf("\n")

	// Check for conflicts
	if hasUnmergedFiles() {
		conflictFiles := getConflictingFiles()
		fmt.Printf("⚠️  Unresolved conflicts (%d):\n", len(conflictFiles))
		for _, file := range conflictFiles {
			fmt.Printf("    ❌ %s\n", file)
		}
		fmt.Printf("\nResolve conflicts, then:\n")
		fmt.Printf("  git add <resolved-files>\n")
		fmt.Printf("  git anticipate --continue\n")
	} else {
		fmt.Printf("✔ All conflicts resolved!\n")
		fmt.Printf("\nRun 'git anticipate --continue' to apply resolution\n")
	}

	fmt.Printf("\nOr run 'git anticipate --abort' to cancel\n")
	return nil
}

// === State Management ===

func isAnticipateInProgress(stateDir string) bool {
	_, err := os.Stat(stateDir)
	return err == nil
}

func saveState(stateDir, targetBranch, origHead, targetSHA, currentBranch string) error {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}

	files := map[string]string{
		"target":         targetBranch,
		"orig_head":      origHead,
		"target_sha":     targetSHA,
		"current_branch": currentBranch,
	}

	for name, content := range files {
		path := filepath.Join(stateDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

func readStateFile(stateDir, name string) (string, error) {
	path := filepath.Join(stateDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func removeState(stateDir string) {
	os.RemoveAll(stateDir)
}

// === Git Operations ===

func validateRepo() error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not a git repository")
	}
	return nil
}

func getGitDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func validateBranchExists(branch string) error {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	return cmd.Run()
}

func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func getRevisionSHA(revision string) (string, error) {
	cmd := exec.Command("git", "rev-parse", revision)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func getMergeBase(branch1, branch2 string) (string, error) {
	cmd := exec.Command("git", "merge-base", branch1, branch2)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func truncateSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 8 {
		return sha[:8]
	}
	if len(sha) == 0 {
		return "unknown"
	}
	return sha
}

func hasUncommittedChanges() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

func hasStagedOrUnstagedChanges() bool {
	// Check for staged changes
	stagedCmd := exec.Command("git", "diff", "--cached", "--quiet")
	if stagedCmd.Run() != nil {
		return true // has staged changes
	}

	// Check for unstaged changes
	unstagedCmd := exec.Command("git", "diff", "--quiet")
	if unstagedCmd.Run() != nil {
		return true // has unstaged changes
	}

	return false
}

// MergeResult represents the outcome of a merge attempt
type MergeResult int

const (
	MergeClean    MergeResult = iota // Merge succeeded with no conflicts
	MergeConflict                    // Merge has conflicts that need resolution
	MergeError                       // Merge failed for other reasons
)

func performMerge(targetBranch string) (MergeResult, error) {
	cmd := exec.Command("git", "merge", targetBranch, "--no-commit", "--no-ff")
	output, err := cmd.CombinedOutput()

	if err != nil {
		if hasUnmergedFiles() {
			return MergeConflict, nil
		}
		return MergeError, fmt.Errorf("merge failed: %s", strings.TrimSpace(string(output)))
	}

	return MergeClean, nil
}

func hasUnmergedFiles() bool {
	cmd := exec.Command("git", "ls-files", "-u")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

func getConflictingFiles() []string {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	output, err := cmd.Output()
	if err != nil {
		return []string{}
	}

	files := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

func abortMerge() {
	cmd := exec.Command("git", "merge", "--abort")
	cmd.Run() // Ignore errors - merge might not be in progress
}
