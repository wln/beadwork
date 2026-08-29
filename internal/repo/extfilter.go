package repo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// bypassedExtensions names git repo extensions that are safe for bw to ignore.
// These don't affect the object format or anything treefs reads; bypassing
// them lets bw open repos that upstream go-git would otherwise reject.
//
// worktreeConfig is set (and core.repositoryformatversion bumped to 1) as
// soon as a user runs `git config --worktree ...` — a common pattern in
// repos that use git worktrees.
var bypassedExtensions = map[string]struct{}{
	"worktreeconfig": {},
}

// openGitRepo opens gitDir with worktreeDir as its worktree, filtering out
// extensions listed in bypassedExtensions so go-git's extension check doesn't
// reject the repo.
func openGitRepo(gitDir, worktreeDir string) (*git.Repository, error) {
	wt := osfs.New(worktreeDir)
	dotGit := osfs.New(gitDir)

	if _, err := dotGit.Stat(""); err != nil {
		if os.IsNotExist(err) {
			return nil, git.ErrRepositoryNotExists
		}
		return nil, err
	}

	objectCache := cache.NewObjectLRUDefault()
	var s storage.Storer = filesystem.NewStorage(dotGit, objectCache)
	if alternates := absoluteAlternateObjectStorers(gitDir, objectCache); len(alternates) > 0 {
		// go-git reconstructs an alternate ObjectStorage on every cache miss.
		// Keeping each one here avoids rebuilding a large pack index for every
		// commit, tree, and blob read. Upstream fixed this for v6 in
		// https://github.com/go-git/go-git/pull/1762; Beadwork remains on v5.
		s = &alternateAwareStorer{
			Storer:     s,
			alternates: alternates,
		}
	}
	return git.Open(&extFilteringStorer{Storer: s}, wt)
}

// absoluteAlternateObjectStorers returns persistent readers when every
// configured object-store alternate is an absolute path. Git commonly writes
// this form for shared clones. Relative alternates retain go-git's existing
// resolution behavior.
func absoluteAlternateObjectStorers(gitDir string, objectCache cache.Object) []storer.EncodedObjectStorer {
	data, err := os.ReadFile(filepath.Join(gitDir, "objects", "info", "alternates"))
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	var alternates []storer.EncodedObjectStorer
	for _, line := range strings.Split(string(data), "\n") {
		path := strings.TrimSuffix(line, "\r")
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			return nil
		}

		path = filepath.Clean(path)
		if filepath.Base(path) != "objects" {
			return nil
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		alternateGitDir := osfs.New(filepath.Dir(path))
		alternates = append(alternates, filesystem.NewStorage(alternateGitDir, objectCache))
	}
	return alternates
}

// alternateAwareStorer keeps alternate object storages alive for the lifetime
// of the repository. Writes and all non-object operations still go only to the
// repository's own storage.
type alternateAwareStorer struct {
	storage.Storer
	alternates []storer.EncodedObjectStorer
}

func (s *alternateAwareStorer) EncodedObject(
	objectType plumbing.ObjectType,
	hash plumbing.Hash,
) (plumbing.EncodedObject, error) {
	object, err := s.Storer.EncodedObject(objectType, hash)
	if !errors.Is(err, plumbing.ErrObjectNotFound) {
		return object, err
	}
	for _, alternate := range s.alternates {
		object, err = alternate.EncodedObject(objectType, hash)
		if err == nil {
			return object, nil
		}
		if !errors.Is(err, plumbing.ErrObjectNotFound) {
			return nil, err
		}
	}
	return nil, plumbing.ErrObjectNotFound
}

func (s *alternateAwareStorer) HasEncodedObject(hash plumbing.Hash) error {
	err := s.Storer.HasEncodedObject(hash)
	if !errors.Is(err, plumbing.ErrObjectNotFound) {
		return err
	}
	for _, alternate := range s.alternates {
		err = alternate.HasEncodedObject(hash)
		if err == nil {
			return nil
		}
		if !errors.Is(err, plumbing.ErrObjectNotFound) {
			return err
		}
	}
	return plumbing.ErrObjectNotFound
}

func (s *alternateAwareStorer) EncodedObjectSize(hash plumbing.Hash) (int64, error) {
	size, err := s.Storer.EncodedObjectSize(hash)
	if !errors.Is(err, plumbing.ErrObjectNotFound) {
		return size, err
	}
	for _, alternate := range s.alternates {
		size, err = alternate.EncodedObjectSize(hash)
		if err == nil {
			return size, nil
		}
		if !errors.Is(err, plumbing.ErrObjectNotFound) {
			return 0, err
		}
	}
	return 0, plumbing.ErrObjectNotFound
}

// extFilteringStorer wraps a storage.Storer and strips bypassed extensions
// from the config returned by Config(). SetConfig is passed through unchanged
// — bw never writes git config, so no round-trip concern here.
type extFilteringStorer struct {
	storage.Storer
}

func (s *extFilteringStorer) Config() (*config.Config, error) {
	cfg, err := s.Storer.Config()
	if err != nil {
		return cfg, err
	}
	stripBypassedExtensions(cfg)
	return cfg, nil
}

func stripBypassedExtensions(cfg *config.Config) {
	if cfg == nil || cfg.Raw == nil || !cfg.Raw.HasSection("extensions") {
		return
	}
	section := cfg.Raw.Section("extensions")
	kept := section.Options[:0]
	for _, opt := range section.Options {
		if _, skip := bypassedExtensions[strings.ToLower(opt.Key)]; skip {
			continue
		}
		kept = append(kept, opt)
	}
	section.Options = kept
}
