package state

import (
	"path/filepath"
	"testing"
)

func TestFileStorePersistsState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store := NewFileStore(path)
	if err := store.Save(State{
		SyncBuf: "cursor-1",
		ContextTokens: map[string]string{
			"wx-user-1": "ctx-1",
		},
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if loaded.SyncBuf != "cursor-1" {
		t.Fatalf("unexpected sync buf: %s", loaded.SyncBuf)
	}

	if loaded.ContextTokens["wx-user-1"] != "ctx-1" {
		t.Fatalf("unexpected context token")
	}
}

func TestFileStoreCanOverwriteExistingState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store := NewFileStore(path)
	if err := store.Save(State{
		SyncBuf: "cursor-1",
		ContextTokens: map[string]string{
			"wx-user-1": "ctx-1",
		},
	}); err != nil {
		t.Fatalf("save initial state: %v", err)
	}

	if err := store.Save(State{
		SyncBuf: "cursor-2",
		ContextTokens: map[string]string{
			"wx-user-1": "ctx-2",
			"wx-user-2": "ctx-3",
		},
	}); err != nil {
		t.Fatalf("overwrite state: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load overwritten state: %v", err)
	}

	if loaded.SyncBuf != "cursor-2" {
		t.Fatalf("unexpected sync buf after overwrite: %s", loaded.SyncBuf)
	}

	if loaded.ContextTokens["wx-user-2"] != "ctx-3" {
		t.Fatalf("unexpected overwritten context token")
	}
}
