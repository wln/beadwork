package repo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jallum/beadwork/internal/repo"
)

func TestFindRepoNotInitialized(t *testing.T) {
	dir := t.TempDir()

	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README"), []byte("test"), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial")

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	r, err := repo.FindRepo()
	if err != nil {
		t.Fatalf("FindRepo: %v", err)
	}
	if r.IsInitialized() {
		t.Error("should not be initialized before Init")
	}
	if r.Prefix != "" {
		t.Errorf("prefix = %q, want empty before init", r.Prefix)
	}
}

func TestFindRepoNotGit(t *testing.T) {
	dir := t.TempDir()

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	_, err := repo.FindRepo()
	if err == nil {
		t.Error("expected error outside git repo")
	}
}

func TestFindRepoAtExplicitDir(t *testing.T) {
	dir := t.TempDir()

	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README"), []byte("test"), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial")

	// No chdir — discover repo purely from the explicit path.
	r, err := repo.FindRepoAt(dir)
	if err != nil {
		t.Fatalf("FindRepoAt: %v", err)
	}
	if r.CWD != dir {
		t.Errorf("CWD = %q, want %q", r.CWD, dir)
	}
	if r.RepoDir() != dir {
		t.Errorf("RepoDir() = %q, want %q", r.RepoDir(), dir)
	}
}

func TestFindRepoAtNestedDir(t *testing.T) {
	dir := t.TempDir()

	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README"), []byte("test"), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial")

	nested := filepath.Join(dir, "a", "b", "c")
	os.MkdirAll(nested, 0755)

	r, err := repo.FindRepoAt(nested)
	if err != nil {
		t.Fatalf("FindRepoAt: %v", err)
	}
	if r.CWD != nested {
		t.Errorf("CWD = %q, want %q", r.CWD, nested)
	}
	if r.RepoDir() != dir {
		t.Errorf("RepoDir() = %q, want %q", r.RepoDir(), dir)
	}
}

func TestFindRepoAtNotGit(t *testing.T) {
	dir := t.TempDir()

	_, err := repo.FindRepoAt(dir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestFindRepoAtEmpty(t *testing.T) {
	// Empty string should fall back to cwd, same as FindRepo.
	dir, _ := filepath.EvalSymlinks(t.TempDir())

	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README"), []byte("test"), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial")

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	r, err := repo.FindRepoAt("")
	if err != nil {
		t.Fatalf("FindRepoAt empty: %v", err)
	}
	if r.RepoDir() != dir {
		t.Errorf("RepoDir() = %q, want %q", r.RepoDir(), dir)
	}
}

func TestFindRepoAtWithWorktreeConfigExtension(t *testing.T) {
	dir := t.TempDir()

	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README"), []byte("test"), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial")

	// Simulate the state real git creates after `git config --worktree ...`:
	// format version bumped to 1, worktreeConfig extension enabled.
	gitRun(t, dir, "config", "core.repositoryFormatVersion", "1")
	gitRun(t, dir, "config", "extensions.worktreeConfig", "true")

	if _, err := repo.FindRepoAt(dir); err != nil {
		t.Fatalf("FindRepoAt with worktreeConfig extension: %v", err)
	}
}

func TestFindRepoAtFromWorktreeWithWorktreeConfig(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	main := filepath.Join(base, "main")
	if err := os.MkdirAll(main, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	gitRun(t, main, "init")
	gitRun(t, main, "config", "user.email", "test@test.com")
	gitRun(t, main, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(main, "README"), []byte("test"), 0644)
	gitRun(t, main, "add", ".")
	gitRun(t, main, "commit", "-m", "initial")

	wt := filepath.Join(base, "wt")
	gitRun(t, main, "worktree", "add", wt)

	// Enable worktreeConfig extension (mirrors what git does automatically
	// when a user runs `git config --worktree ...`).
	gitRun(t, main, "config", "core.repositoryFormatVersion", "1")
	gitRun(t, main, "config", "extensions.worktreeConfig", "true")
	gitRun(t, wt, "config", "--worktree", "core.sparseCheckout", "true")

	r, err := repo.FindRepoAt(wt)
	if err != nil {
		t.Fatalf("FindRepoAt from worktree: %v", err)
	}
	if r.RepoDir() != main {
		t.Errorf("RepoDir() = %q, want %q", r.RepoDir(), main)
	}
}

func TestFindRepoAtSeparateGitDirWithSharedObjects(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	source := filepath.Join(base, "source")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("MkdirAll source: %v", err)
	}

	gitRun(t, source, "init")
	gitRun(t, source, "config", "user.email", "test@test.com")
	gitRun(t, source, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(source, "README"), []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile README: %v", err)
	}
	gitRun(t, source, "add", ".")
	gitRun(t, source, "commit", "-m", "initial")

	sourceRepo, err := repo.FindRepoAt(source)
	if err != nil {
		t.Fatalf("FindRepoAt source: %v", err)
	}
	if err := sourceRepo.Init("test", nil); err != nil {
		t.Fatalf("Init source: %v", err)
	}
	// Exercise packed alternate storage, where rebuilding go-git's in-memory
	// pack index for every object lookup is prohibitively expensive at scale.
	gitRun(t, source, "gc", "--prune=now")

	gitDir := filepath.Join(base, "git-dirs", "repo.git")
	worktree := filepath.Join(base, "worktree")
	if err := os.MkdirAll(filepath.Dir(gitDir), 0755); err != nil {
		t.Fatalf("MkdirAll git-dirs: %v", err)
	}
	gitRun(t, base, "clone", "--shared", "--separate-git-dir", gitDir, source, worktree)
	gitRun(t, worktree, "branch", repo.BranchName, "origin/"+repo.BranchName)

	alternates, err := os.ReadFile(filepath.Join(gitDir, "objects", "info", "alternates"))
	if err != nil {
		t.Fatalf("ReadFile alternates: %v", err)
	}
	if alternate := strings.TrimSpace(string(alternates)); !filepath.IsAbs(alternate) {
		t.Fatalf("alternate path = %q, want absolute path", alternate)
	}

	nested := filepath.Join(worktree, "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("MkdirAll nested: %v", err)
	}
	r, err := repo.FindRepoAt(nested)
	if err != nil {
		t.Fatalf("FindRepoAt separate git dir: %v", err)
	}
	if !r.IsInitialized() {
		t.Fatal("repo should be initialized from shared beadwork branch")
	}
	if r.Prefix != "test" {
		t.Errorf("prefix = %q, want test", r.Prefix)
	}
	if r.GitDir != gitDir {
		t.Errorf("GitDir = %q, want %q", r.GitDir, gitDir)
	}
	if r.RepoDir() != worktree {
		t.Errorf("RepoDir() = %q, want %q", r.RepoDir(), worktree)
	}

	if err := r.TreeFS().WriteFile("issues/separate.json", []byte(`{"id":"separate"}`)); err != nil {
		t.Fatalf("WriteFile issue: %v", err)
	}
	if err := r.Commit("write through separate git dir"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if _, err := r.Reopen(); err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	commits, err := r.AllCommits()
	if err != nil {
		t.Fatalf("AllCommits: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("len(AllCommits()) = %d, want 2", len(commits))
	}

	newHash := gitOutput(t, worktree, "rev-parse", repo.BranchName)
	if err := gitObjectExists(source, newHash); err == nil {
		t.Fatalf("new commit %s unexpectedly written to shared source", newHash)
	}
	if err := gitObjectExists(worktree, newHash); err != nil {
		t.Fatalf("new commit %s missing from separate git dir: %v", newHash, err)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
	return strings.TrimSpace(string(out))
}

func gitObjectExists(dir, hash string) error {
	cmd := exec.Command("git", "cat-file", "-e", hash+"^{commit}")
	cmd.Dir = dir
	return cmd.Run()
}

func TestInitInvalidPrefix(t *testing.T) {
	dir := t.TempDir()

	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README"), []byte("test"), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial")

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	r, _ := repo.FindRepo()
	err := r.Init("has space", nil)
	if err == nil {
		t.Error("expected error for invalid prefix")
	}
}

func TestDerivePrefixFallback(t *testing.T) {
	// Create a repo in a directory named with only special chars
	base := t.TempDir()
	dir := filepath.Join(base, "...")
	os.Mkdir(dir, 0755)

	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README"), []byte("test"), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial")

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	r, _ := repo.FindRepo()
	if err := r.Init("", nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// Should fallback to "bw" when dir name has no valid chars
	if r.Prefix != "bw" {
		t.Errorf("prefix = %q, want bw", r.Prefix)
	}
}

func TestCommitWithChanges(t *testing.T) {
	dir := t.TempDir()

	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README"), []byte("test"), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial")

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	r, _ := repo.FindRepo()
	r.Init("test", nil)

	// Create a file and commit
	r.TreeFS().WriteFile("issues/test.json", []byte(`{"id":"test"}`))
	if err := r.Commit("test commit"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Commit with no changes should be a noop (not an error)
	if err := r.Commit("noop"); err != nil {
		t.Fatalf("Commit noop: %v", err)
	}
}
