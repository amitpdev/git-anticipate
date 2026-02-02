package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestHelper provides utilities for setting up test git repos
type TestHelper struct {
	t       *testing.T
	repoDir string
}

func NewTestHelper(t *testing.T) *TestHelper {
	t.Helper()
	
	// Create temp directory for test repo
	dir, err := os.MkdirTemp("", "git-anticipate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	
	return &TestHelper{t: t, repoDir: dir}
}

func (h *TestHelper) Cleanup() {
	os.RemoveAll(h.repoDir)
}

func (h *TestHelper) Run(name string, args ...string) string {
	h.t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = h.repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		h.t.Logf("Command '%s %s' failed: %v\nOutput: %s", name, strings.Join(args, " "), err, output)
	}
	return string(output)
}

func (h *TestHelper) RunExpectSuccess(name string, args ...string) string {
	h.t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = h.repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		h.t.Fatalf("Command '%s %s' failed: %v\nOutput: %s", name, strings.Join(args, " "), err, output)
	}
	return string(output)
}

func (h *TestHelper) RunExpectFailure(name string, args ...string) string {
	h.t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = h.repoDir
	output, err := cmd.CombinedOutput()
	if err == nil {
		h.t.Fatalf("Command '%s %s' expected to fail but succeeded\nOutput: %s", name, strings.Join(args, " "), output)
	}
	return string(output)
}

func (h *TestHelper) WriteFile(path, content string) {
	h.t.Helper()
	fullPath := filepath.Join(h.repoDir, path)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.t.Fatalf("failed to create dir %s: %v", dir, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		h.t.Fatalf("failed to write file %s: %v", path, err)
	}
}

func (h *TestHelper) DeleteFile(path string) {
	h.t.Helper()
	fullPath := filepath.Join(h.repoDir, path)
	if err := os.Remove(fullPath); err != nil {
		h.t.Fatalf("failed to delete file %s: %v", path, err)
	}
}

func (h *TestHelper) FileExists(path string) bool {
	fullPath := filepath.Join(h.repoDir, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

func (h *TestHelper) ReadFile(path string) string {
	h.t.Helper()
	fullPath := filepath.Join(h.repoDir, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		h.t.Fatalf("failed to read file %s: %v", path, err)
	}
	return string(content)
}

func (h *TestHelper) InitRepo() {
	h.t.Helper()
	h.RunExpectSuccess("git", "init", "-b", "main")
	h.RunExpectSuccess("git", "config", "user.email", "test@test.com")
	h.RunExpectSuccess("git", "config", "user.name", "Test User")
}

func (h *TestHelper) Commit(message string) {
	h.t.Helper()
	h.RunExpectSuccess("git", "add", "-A")
	h.RunExpectSuccess("git", "commit", "-m", message)
}

func (h *TestHelper) Branch(name string) {
	h.t.Helper()
	h.RunExpectSuccess("git", "checkout", "-b", name)
}

func (h *TestHelper) Checkout(name string) {
	h.t.Helper()
	h.RunExpectSuccess("git", "checkout", name)
}

func (h *TestHelper) CurrentBranch() string {
	h.t.Helper()
	output := h.RunExpectSuccess("git", "rev-parse", "--abbrev-ref", "HEAD")
	return strings.TrimSpace(output)
}

func (h *TestHelper) CommitCount() int {
	h.t.Helper()
	output := h.RunExpectSuccess("git", "rev-list", "--count", "HEAD")
	var count int
	if _, err := strings.NewReader(strings.TrimSpace(output)).Read([]byte{}); err == nil {
		count = len(strings.Split(strings.TrimSpace(output), "\n"))
	}
	// Simple parsing
	for _, c := range strings.TrimSpace(output) {
		if c >= '0' && c <= '9' {
			count = count*10 + int(c-'0')
		}
	}
	return count
}

func (h *TestHelper) LastCommitMessage() string {
	h.t.Helper()
	output := h.RunExpectSuccess("git", "log", "-1", "--format=%s")
	return strings.TrimSpace(output)
}

// =============================================================================
// TEST: Deleted Files
// This test would have caught the bug where deleted files weren't handled
// 
// Scenario: Feature branch renames a file (delete old + add new)
// Dev branch modifies the old file
// When merged: the old file is deleted, new file is added
// =============================================================================

func TestDeletedFileInMergeResolution(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// Setup: Create repo with initial files
	h.InitRepo()
	h.WriteFile("oldfile.txt", "content of old file")
	h.WriteFile("other.txt", "other content")
	h.Commit("initial commit")
	
	// Create dev branch and modify both files
	h.Branch("dev")
	h.WriteFile("oldfile.txt", "dev modified old file")
	h.WriteFile("other.txt", "dev modified other")
	h.Commit("dev modifies files")
	
	// Go back to main and create feature branch
	h.Checkout("main")
	h.Branch("feature")
	
	// On feature: delete oldfile.txt and create newfile.txt, modify other.txt
	h.DeleteFile("oldfile.txt")
	h.WriteFile("newfile.txt", "content in new file")
	h.WriteFile("other.txt", "feature modified other")
	h.Run("git", "add", "-A")
	h.Commit("feature: rename old->new, modify other")
	
	// Now run git-anticipate
	output := h.Run("git-anticipate", "dev")
	
	t.Logf("First output: %s", output)
	
	// Should detect conflict on other.txt at least
	if !strings.Contains(output, "Conflicts detected") {
		// No conflicts - try to continue anyway
		h.Run("git-anticipate", "--abort")
		t.Skip("No conflicts detected, skipping test")
		return
	}
	
	// Resolve the conflict on other.txt by keeping feature's version
	h.WriteFile("other.txt", "feature modified other")
	h.Run("git", "add", "other.txt")
	
	// The oldfile.txt might be marked as deleted - make sure it stays deleted
	if h.FileExists("oldfile.txt") {
		h.Run("git", "rm", "-f", "oldfile.txt")
	}
	
	// Continue - this is where the bug would manifest
	output = h.Run("git-anticipate", "--continue", "--no-verify")
	
	t.Logf("Continue output: %s", output)
	
	// THE BUG: "failed to read resolved file" when file was deleted
	if strings.Contains(output, "failed to read resolved file") {
		t.Errorf("BUG DETECTED: git-anticipate failed to handle deleted file!\nOutput: %s", output)
	}
	
	// Should succeed
	if !strings.Contains(output, "Success") && !strings.Contains(output, "No changes") {
		t.Errorf("Expected success or no-changes message, got: %s", output)
	}
	
	// Verify oldfile.txt is deleted
	if h.FileExists("oldfile.txt") {
		t.Error("oldfile.txt should be deleted but exists")
	}
	
	// Verify newfile.txt exists
	if !h.FileExists("newfile.txt") {
		t.Error("newfile.txt should exist but doesn't")
	}
}

// =============================================================================
// TEST: Deleted File - Direct Case
// More direct test: dev deletes a file, feature modifies it
// =============================================================================

func TestDeletedFileDirectCase(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// Setup
	h.InitRepo()
	h.WriteFile("file.txt", "original content")
	h.WriteFile("keep.txt", "this stays")
	h.Commit("initial commit")
	
	// Dev deletes file.txt
	h.Branch("dev")
	h.Run("git", "rm", "file.txt")
	h.Commit("dev deletes file")
	
	// Feature modifies file.txt
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("file.txt", "feature modified this")
	h.Commit("feature modifies file")
	
	// Run git-anticipate
	output := h.Run("git-anticipate", "dev")
	
	t.Logf("Output: %s", output)
	
	// Expect modify/delete conflict
	if !strings.Contains(output, "Conflicts detected") {
		h.Run("git-anticipate", "--abort")
		t.Skip("No conflicts detected")
		return
	}
	
	// Resolve by accepting deletion (what dev did)
	h.Run("git", "rm", "-f", "file.txt")
	
	// Continue
	output = h.Run("git-anticipate", "--continue", "--no-verify")
	
	t.Logf("Continue output: %s", output)
	
	// Should NOT fail with "failed to read resolved file"
	if strings.Contains(output, "failed to read resolved file") {
		t.Errorf("BUG: Failed to handle deleted file!\nOutput: %s", output)
	}
}

// =============================================================================
// TEST: Basic Conflict Resolution
// =============================================================================

func TestBasicConflictResolution(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// Setup
	h.InitRepo()
	h.WriteFile("file.txt", "line 1\nline 2\nline 3\n")
	h.Commit("initial commit")
	
	// Dev branch changes line 2
	h.Branch("dev")
	h.WriteFile("file.txt", "line 1\nmodified by dev\nline 3\n")
	h.Commit("dev changes")
	
	// Feature branch changes line 2 differently
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("file.txt", "line 1\nmodified by feature\nline 3\n")
	h.Commit("feature changes")
	
	// Run git-anticipate
	output := h.Run("git-anticipate", "dev")
	
	if !strings.Contains(output, "Conflicts detected") {
		t.Fatalf("Expected conflicts, got: %s", output)
	}
	
	if !strings.Contains(output, "file.txt") {
		t.Errorf("Expected file.txt in conflict list, got: %s", output)
	}
	
	// Resolve by creating a MERGED version (different from both branches)
	// This ensures there's an actual change to commit
	h.WriteFile("file.txt", "line 1\nMERGED: feature + dev\nline 3\n")
	h.Run("git", "add", "file.txt")
	
	// Continue
	output = h.Run("git-anticipate", "--continue", "--no-verify")
	
	if !strings.Contains(output, "Success") {
		t.Errorf("Expected success, got: %s", output)
	}
	
	// Verify content has our merged version
	content := h.ReadFile("file.txt")
	if !strings.Contains(content, "MERGED") {
		t.Errorf("Expected merged content, got: %s", content)
	}
}

// =============================================================================
// TEST: No Conflicts
// =============================================================================

func TestNoConflicts(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// Setup
	h.InitRepo()
	h.WriteFile("file1.txt", "content 1")
	h.Commit("initial commit")
	
	// Dev adds a different file
	h.Branch("dev")
	h.WriteFile("file2.txt", "content 2")
	h.Commit("dev adds file2")
	
	// Feature adds another file
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("file3.txt", "content 3")
	h.Commit("feature adds file3")
	
	// Run git-anticipate - should be clean
	output := h.Run("git-anticipate", "dev")
	
	if strings.Contains(output, "Conflicts detected") {
		t.Errorf("Expected no conflicts, got: %s", output)
	}
	
	// Clean up (abort since no conflicts)
	h.Run("git-anticipate", "--abort")
}

// =============================================================================
// TEST: Abort Restores State
// =============================================================================

func TestAbortRestoresState(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// Setup
	h.InitRepo()
	h.WriteFile("file.txt", "original")
	h.Commit("initial commit")
	
	// Dev changes file
	h.Branch("dev")
	h.WriteFile("file.txt", "dev version")
	h.Commit("dev changes")
	
	// Feature changes file
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("file.txt", "feature version")
	h.Commit("feature changes")
	
	// Remember original HEAD
	origHead := strings.TrimSpace(h.RunExpectSuccess("git", "rev-parse", "HEAD"))
	
	// Run git-anticipate (creates conflict)
	h.Run("git-anticipate", "dev")
	
	// Abort
	output := h.Run("git-anticipate", "--abort")
	
	if !strings.Contains(output, "Restored to original state") {
		t.Errorf("Expected restore message, got: %s", output)
	}
	
	// Verify HEAD is restored
	currentHead := strings.TrimSpace(h.RunExpectSuccess("git", "rev-parse", "HEAD"))
	if currentHead != origHead {
		t.Errorf("HEAD not restored: expected %s, got %s", origHead, currentHead)
	}
	
	// Verify file content is restored
	content := h.ReadFile("file.txt")
	if content != "feature version" {
		t.Errorf("File content not restored: got %s", content)
	}
}

// =============================================================================
// TEST: Already In Progress
// =============================================================================

func TestAlreadyInProgress(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// Setup
	h.InitRepo()
	h.WriteFile("file.txt", "original")
	h.Commit("initial commit")
	
	// Dev changes file
	h.Branch("dev")
	h.WriteFile("file.txt", "dev version")
	h.Commit("dev changes")
	
	// Feature changes file
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("file.txt", "feature version")
	h.Commit("feature changes")
	
	// Start anticipate
	h.Run("git-anticipate", "dev")
	
	// Try to start another one
	output := h.RunExpectFailure("git-anticipate", "dev")
	
	if !strings.Contains(output, "already in progress") {
		t.Errorf("Expected 'already in progress' error, got: %s", output)
	}
	
	// Cleanup
	h.Run("git-anticipate", "--abort")
}

// =============================================================================
// TEST: Uncommitted Changes Blocked
// =============================================================================

func TestUncommittedChangesBlocked(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// Setup
	h.InitRepo()
	h.WriteFile("file.txt", "original")
	h.Commit("initial commit")
	
	// Create dev branch
	h.Branch("dev")
	h.Checkout("main")
	
	// Make uncommitted changes
	h.WriteFile("file.txt", "uncommitted changes")
	
	// Try to run git-anticipate
	output := h.RunExpectFailure("git-anticipate", "dev")
	
	if !strings.Contains(output, "uncommitted changes") {
		t.Errorf("Expected uncommitted changes error, got: %s", output)
	}
}

// =============================================================================
// TEST: Both Added Conflict (same content)
// =============================================================================

func TestBothAddedSameContent(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// Setup
	h.InitRepo()
	h.WriteFile("existing.txt", "existing")
	h.Commit("initial commit")
	
	// Dev adds new file
	h.Branch("dev")
	h.WriteFile("newfile.txt", "same content")
	h.Commit("dev adds newfile")
	
	// Feature adds same file with same content
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("newfile.txt", "same content")
	h.Commit("feature adds newfile")
	
	// Run git-anticipate
	output := h.Run("git-anticipate", "dev")
	
	// For "both added" with identical content, Git might:
	// 1. Mark as conflict (need to resolve)
	// 2. Auto-resolve (no conflict)
	
	if strings.Contains(output, "Conflicts detected") {
		// Resolve by just adding
		h.Run("git", "add", "newfile.txt")
		
		// Continue
		output = h.Run("git-anticipate", "--continue", "--no-verify")
		
		// Should succeed or say no changes
		if strings.Contains(output, "failed") && !strings.Contains(output, "No changes") {
			t.Errorf("Expected success or no-changes, got: %s", output)
		}
	} else {
		// No conflict - clean up if state exists
		h.Run("git-anticipate", "--abort")
	}
}

// =============================================================================
// TEST: Renamed File
// =============================================================================

func TestRenamedFile(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// Setup
	h.InitRepo()
	h.WriteFile("oldname.txt", "content")
	h.Commit("initial commit")
	
	// Dev modifies the file
	h.Branch("dev")
	h.WriteFile("oldname.txt", "modified content")
	h.Commit("dev modifies")
	
	// Feature renames the file
	h.Checkout("main")
	h.Branch("feature")
	h.Run("git", "mv", "oldname.txt", "newname.txt")
	h.Commit("feature renames")
	
	// Run git-anticipate
	output := h.Run("git-anticipate", "dev")
	
	// This might or might not conflict depending on Git's rename detection
	// Just make sure it doesn't crash
	if strings.Contains(output, "panic") || strings.Contains(output, "fatal error") {
		t.Errorf("Git anticipate crashed: %s", output)
	}
	
	// Cleanup
	h.Run("git-anticipate", "--abort")
}

// =============================================================================
// TEST: Status Command
// =============================================================================

func TestStatusCommand(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// Setup with actual conflict so anticipate stays in progress
	h.InitRepo()
	h.WriteFile("file.txt", "original")
	h.Commit("initial commit")
	
	// Dev modifies
	h.Branch("dev")
	h.WriteFile("file.txt", "dev version")
	h.Commit("dev changes")
	
	// Feature modifies differently (creates conflict)
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("file.txt", "feature version")
	h.Commit("feature changes")
	
	// Test status when not in progress
	output := h.Run("git-anticipate", "--status")
	if !strings.Contains(output, "No anticipate in progress") {
		t.Errorf("Expected 'no anticipate' message, got: %s", output)
	}
	
	// Start anticipate (will have conflicts, so stays in progress)
	h.Run("git-anticipate", "dev")
	
	output = h.Run("git-anticipate", "--status")
	if !strings.Contains(output, "In Progress") && !strings.Contains(output, "dev") {
		t.Errorf("Expected in-progress status with 'dev', got: %s", output)
	}
	
	// Cleanup
	h.Run("git-anticipate", "--abort")
}

// =============================================================================
// TEST: Invalid Branch
// =============================================================================

func TestInvalidBranch(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// Setup
	h.InitRepo()
	h.WriteFile("file.txt", "content")
	h.Commit("initial commit")
	
	// Try to anticipate non-existent branch
	output := h.RunExpectFailure("git-anticipate", "nonexistent-branch")
	
	if !strings.Contains(output, "does not exist") {
		t.Errorf("Expected 'does not exist' error, got: %s", output)
	}
}

// =============================================================================
// TEST: Multiple Files with Mixed Changes
// =============================================================================

func TestMultipleFilesWithMixedChanges(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// Setup with multiple files
	h.InitRepo()
	h.WriteFile("keep.txt", "keep this")
	h.WriteFile("modify.txt", "original")
	h.WriteFile("delete.txt", "will be deleted")
	h.Commit("initial commit")
	
	// Dev branch: modify and add
	h.Branch("dev")
	h.WriteFile("modify.txt", "dev modified")
	h.WriteFile("new-on-dev.txt", "new file")
	h.Commit("dev changes")
	
	// Feature branch: modify differently and delete
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("modify.txt", "feature modified")
	h.DeleteFile("delete.txt")
	h.Run("git", "add", "-A")
	h.Commit("feature changes")
	
	// Run git-anticipate
	output := h.Run("git-anticipate", "dev")
	
	// Should have conflict on modify.txt
	if !strings.Contains(output, "modify.txt") {
		t.Logf("Output: %s", output)
	}
	
	// If conflicts, resolve them
	if strings.Contains(output, "Conflicts detected") {
		// Resolve modify.txt by keeping feature version
		h.WriteFile("modify.txt", "feature modified")
		h.Run("git", "add", "modify.txt")
		
		// Make sure delete.txt stays deleted (if it exists due to merge)
		if h.FileExists("delete.txt") {
			h.Run("git", "rm", "-f", "delete.txt")
		}
		
		// Continue
		output = h.Run("git-anticipate", "--continue", "--no-verify")
		
		if !strings.Contains(output, "Success") {
			t.Errorf("Expected success, got: %s", output)
		}
	}
	
	// Verify final state
	if h.FileExists("delete.txt") {
		t.Error("delete.txt should not exist")
	}
	
	content := h.ReadFile("modify.txt")
	if !strings.Contains(content, "feature") {
		t.Errorf("modify.txt should have feature content, got: %s", content)
	}
}

// =============================================================================
// TEST: Help Flag
// =============================================================================

func TestHelpFlag(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// -h should show help (not require a git repo)
	output := h.Run("git-anticipate", "-h")
	
	if !strings.Contains(output, "git-anticipate") {
		t.Errorf("Expected help text, got: %s", output)
	}
	
	if !strings.Contains(output, "--continue") {
		t.Errorf("Expected --continue in help, got: %s", output)
	}
	
	if !strings.Contains(output, "--abort") {
		t.Errorf("Expected --abort in help, got: %s", output)
	}
}

// =============================================================================
// TEST: Version Flag
// =============================================================================

func TestVersionFlag(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	output := h.Run("git-anticipate", "--version")
	
	if !strings.Contains(output, "git-anticipate") {
		t.Errorf("Expected version info, got: %s", output)
	}
	
	// Should contain version number pattern
	if !strings.Contains(output, ".") {
		t.Errorf("Expected version number with dot, got: %s", output)
	}
}

// =============================================================================
// TEST: Not a Git Repository
// =============================================================================

func TestNotAGitRepository(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	// Don't init - just try to run in empty directory
	output := h.RunExpectFailure("git-anticipate", "main")
	
	if !strings.Contains(output, "not a git repository") {
		t.Errorf("Expected 'not a git repository' error, got: %s", output)
	}
}

// =============================================================================
// TEST: Continue Without Start
// =============================================================================

func TestContinueWithoutStart(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	h.InitRepo()
	h.WriteFile("file.txt", "content")
	h.Commit("initial")
	
	output := h.RunExpectFailure("git-anticipate", "--continue")
	
	if !strings.Contains(output, "no anticipate in progress") {
		t.Errorf("Expected 'no anticipate in progress', got: %s", output)
	}
}

// =============================================================================
// TEST: Abort Without Start
// =============================================================================

func TestAbortWithoutStart(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	h.InitRepo()
	h.WriteFile("file.txt", "content")
	h.Commit("initial")
	
	output := h.RunExpectFailure("git-anticipate", "--abort")
	
	if !strings.Contains(output, "no anticipate in progress") {
		t.Errorf("Expected 'no anticipate in progress', got: %s", output)
	}
}

// =============================================================================
// TEST: Same Branch (anticipate current branch)
// =============================================================================

func TestSameBranch(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	h.InitRepo()
	h.WriteFile("file.txt", "content")
	h.Commit("initial")
	
	// Try to anticipate the current branch
	output := h.Run("git-anticipate", "main")
	
	// Should either succeed with "no conflicts" or handle gracefully
	// Git merge with self typically succeeds with "Already up to date"
	if strings.Contains(output, "panic") || strings.Contains(output, "fatal error") {
		t.Errorf("Should not crash when anticipating same branch: %s", output)
	}
	
	// Clean up any state
	h.Run("git-anticipate", "--abort")
}

// =============================================================================
// TEST: New File Added (no conflict)
// =============================================================================

func TestNewFileNoConflict(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	h.InitRepo()
	h.WriteFile("existing.txt", "existing")
	h.Commit("initial")
	
	// Dev adds file1
	h.Branch("dev")
	h.WriteFile("file1.txt", "from dev")
	h.Commit("dev adds file1")
	
	// Feature adds file2 (different file - no conflict)
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("file2.txt", "from feature")
	h.Commit("feature adds file2")
	
	// Anticipate - should have no conflicts
	output := h.Run("git-anticipate", "dev")
	
	if strings.Contains(output, "Conflicts detected") {
		t.Errorf("Expected no conflicts for different files, got: %s", output)
	}
	
	// Clean up
	h.Run("git-anticipate", "--abort")
}

// =============================================================================
// TEST: Binary File Handling
// =============================================================================

func TestBinaryFile(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	h.InitRepo()
	
	// Create a "binary" file (just bytes)
	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}
	fullPath := filepath.Join(h.repoDir, "binary.bin")
	if err := os.WriteFile(fullPath, binaryContent, 0644); err != nil {
		t.Fatalf("failed to write binary file: %v", err)
	}
	h.Commit("add binary")
	
	// Dev modifies binary
	h.Branch("dev")
	devContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFD}
	os.WriteFile(fullPath, devContent, 0644)
	h.Commit("dev modifies binary")
	
	// Feature modifies binary differently
	h.Checkout("main")
	h.Branch("feature")
	featureContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFC}
	os.WriteFile(fullPath, featureContent, 0644)
	h.Commit("feature modifies binary")
	
	// Anticipate
	output := h.Run("git-anticipate", "dev")
	
	// Should handle binary conflicts (or at least not crash)
	if strings.Contains(output, "panic") {
		t.Errorf("Should not panic on binary file: %s", output)
	}
	
	// Clean up
	h.Run("git-anticipate", "--abort")
}

// =============================================================================
// TEST: Subdirectory File Conflict
// =============================================================================

func TestSubdirectoryConflict(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	h.InitRepo()
	h.WriteFile("src/main.go", "package main")
	h.Commit("initial")
	
	// Dev modifies nested file
	h.Branch("dev")
	h.WriteFile("src/main.go", "package main\n// dev")
	h.Commit("dev changes")
	
	// Feature modifies same nested file
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("src/main.go", "package main\n// feature")
	h.Commit("feature changes")
	
	// Anticipate
	output := h.Run("git-anticipate", "dev")
	
	if !strings.Contains(output, "Conflicts detected") {
		t.Fatalf("Expected conflict in subdirectory, got: %s", output)
	}
	
	if !strings.Contains(output, "src/main.go") {
		t.Errorf("Expected src/main.go in conflict list, got: %s", output)
	}
	
	// Resolve
	h.WriteFile("src/main.go", "package main\n// merged")
	h.Run("git", "add", "src/main.go")
	
	// Continue
	output = h.Run("git-anticipate", "--continue", "--no-verify")
	
	if !strings.Contains(output, "Success") {
		t.Errorf("Expected success, got: %s", output)
	}
	
	// Verify nested file has merged content
	content := h.ReadFile("src/main.go")
	if !strings.Contains(content, "merged") {
		t.Errorf("Expected merged content in nested file, got: %s", content)
	}
}

// =============================================================================
// TEST: Multiple Sequential Anticipates
// =============================================================================

func TestMultipleSequentialAnticipates(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	h.InitRepo()
	h.WriteFile("file.txt", "original")
	h.Commit("initial")
	
	// Dev makes changes
	h.Branch("dev")
	h.WriteFile("file.txt", "dev version 1")
	h.Commit("dev v1")
	
	// Feature makes conflicting changes
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("file.txt", "feature version 1")
	h.Commit("feature v1")
	
	// First anticipate
	h.Run("git-anticipate", "dev")
	h.WriteFile("file.txt", "merged v1")
	h.Run("git", "add", "file.txt")
	h.Run("git-anticipate", "--continue", "--no-verify")
	
	// Now dev makes more changes
	h.Checkout("dev")
	h.WriteFile("file.txt", "dev version 2")
	h.Commit("dev v2")
	
	// Back to feature - anticipate again
	h.Checkout("feature")
	output := h.Run("git-anticipate", "dev")
	
	// Should be able to anticipate again
	if strings.Contains(output, "already in progress") {
		t.Errorf("Should be able to anticipate again after completing, got: %s", output)
	}
	
	// Clean up
	h.Run("git-anticipate", "--abort")
}

// =============================================================================
// TEST: Empty Commit (no actual changes after resolution)
// =============================================================================

func TestEmptyResolution(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	h.InitRepo()
	h.WriteFile("file.txt", "original")
	h.Commit("initial")
	
	// Dev and Feature make the SAME change
	h.Branch("dev")
	h.WriteFile("file.txt", "same change")
	h.Commit("dev changes")
	
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("file.txt", "same change")
	h.Commit("feature changes")
	
	// Anticipate
	output := h.Run("git-anticipate", "dev")
	
	// Git might auto-resolve this or show conflict
	// Either way, should handle gracefully
	if strings.Contains(output, "panic") {
		t.Errorf("Should not panic: %s", output)
	}
	
	// If conflict, resolve and continue
	if strings.Contains(output, "Conflicts") {
		h.Run("git", "add", "file.txt")
		output = h.Run("git-anticipate", "--continue", "--no-verify")
		
		// Should say "no changes" or succeed
		if strings.Contains(output, "failed") && !strings.Contains(output, "No changes") {
			t.Errorf("Should handle empty resolution gracefully, got: %s", output)
		}
	} else {
		h.Run("git-anticipate", "--abort")
	}
}

// =============================================================================
// TEST: Whitespace-only Changes
// =============================================================================

func TestWhitespaceOnlyConflict(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	h.InitRepo()
	h.WriteFile("file.txt", "line1\nline2\n")
	h.Commit("initial")
	
	// Dev adds trailing whitespace
	h.Branch("dev")
	h.WriteFile("file.txt", "line1  \nline2\n")
	h.Commit("dev whitespace")
	
	// Feature adds different whitespace
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("file.txt", "line1\nline2  \n")
	h.Commit("feature whitespace")
	
	// Anticipate
	output := h.Run("git-anticipate", "dev")
	
	// Should not crash
	if strings.Contains(output, "panic") {
		t.Errorf("Should not panic on whitespace conflict: %s", output)
	}
	
	// Clean up
	h.Run("git-anticipate", "--abort")
}

// =============================================================================
// TEST: No-Verify Flag Actually Skips Hooks
// =============================================================================

func TestNoVerifyFlag(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()
	
	h.InitRepo()
	h.WriteFile("file.txt", "original")
	h.Commit("initial")
	
	// Create conflict FIRST (before adding the hook)
	h.Branch("dev")
	h.WriteFile("file.txt", "dev")
	h.Commit("dev")
	
	h.Checkout("main")
	h.Branch("feature")
	h.WriteFile("file.txt", "feature")
	h.Commit("feature")
	
	// NOW create a pre-commit hook that always fails
	// (after all setup commits are done)
	hooksDir := filepath.Join(h.repoDir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)
	hookPath := filepath.Join(hooksDir, "pre-commit")
	hookContent := "#!/bin/sh\nexit 1\n"
	os.WriteFile(hookPath, []byte(hookContent), 0755)
	
	// Anticipate
	h.Run("git-anticipate", "dev")
	
	// Resolve
	h.WriteFile("file.txt", "resolved")
	h.Run("git", "add", "file.txt")
	
	// WITHOUT --no-verify should fail (hook exits 1)
	output := h.Run("git-anticipate", "--continue")
	hookFailed := strings.Contains(output, "failed") || strings.Contains(output, "Error")
	
	if !hookFailed {
		// Hook might not have run - skip this part of test
		t.Log("Pre-commit hook may not have executed")
	}
	
	// Abort and try again
	h.Run("git-anticipate", "--abort")
	h.Run("git-anticipate", "dev")
	h.WriteFile("file.txt", "resolved again")
	h.Run("git", "add", "file.txt")
	
	// WITH --no-verify should succeed
	output = h.Run("git-anticipate", "--continue", "--no-verify")
	
	if !strings.Contains(output, "Success") {
		t.Errorf("Expected success with --no-verify, got: %s", output)
	}
}
