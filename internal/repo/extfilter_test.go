package repo

import (
	"errors"
	"io"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage/memory"
)

func TestAlternateAwareStorerReadsAlternateAndWritesLocal(t *testing.T) {
	local := memory.NewStorage()
	alternate := memory.NewStorage()
	alternateHash := storeBlob(t, alternate, "from alternate")

	s := &alternateAwareStorer{
		Storer:     local,
		alternates: []storer.EncodedObjectStorer{alternate},
	}

	object, err := s.EncodedObject(plumbing.BlobObject, alternateHash)
	if err != nil {
		t.Fatalf("EncodedObject from alternate: %v", err)
	}
	reader, err := object.Reader()
	if err != nil {
		t.Fatalf("Reader: %v", err)
	}
	data, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if got := string(data); got != "from alternate" {
		t.Errorf("object contents = %q, want %q", got, "from alternate")
	}
	if err := s.HasEncodedObject(alternateHash); err != nil {
		t.Fatalf("HasEncodedObject from alternate: %v", err)
	}
	if size, err := s.EncodedObjectSize(alternateHash); err != nil {
		t.Fatalf("EncodedObjectSize from alternate: %v", err)
	} else if size != int64(len("from alternate")) {
		t.Errorf("EncodedObjectSize = %d, want %d", size, len("from alternate"))
	}

	localHash := storeBlob(t, s, "local only")
	if err := local.HasEncodedObject(localHash); err != nil {
		t.Fatalf("local object missing from local storage: %v", err)
	}
	if err := alternate.HasEncodedObject(localHash); !errors.Is(err, plumbing.ErrObjectNotFound) {
		t.Fatalf("alternate HasEncodedObject(local hash) error = %v, want object not found", err)
	}
}

func storeBlob(t *testing.T, destination storer.EncodedObjectStorer, contents string) plumbing.Hash {
	t.Helper()
	object := destination.NewEncodedObject()
	object.SetType(plumbing.BlobObject)
	object.SetSize(int64(len(contents)))
	writer, err := object.Writer()
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}
	if _, err := io.WriteString(writer, contents); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}
	hash, err := destination.SetEncodedObject(object)
	if err != nil {
		t.Fatalf("SetEncodedObject: %v", err)
	}
	return hash
}
