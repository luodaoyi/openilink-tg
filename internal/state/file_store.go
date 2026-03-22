package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// State 保存服务运行所需的最小持久化数据。
type State struct {
	SyncBuf       string            `json:"sync_buf"`
	ContextTokens map[string]string `json:"context_tokens"`
}

// FileStore 以 JSON 文件持久化运行状态。
type FileStore struct {
	path string
	mu   sync.Mutex
}

// NewFileStore 创建一个基于文件的状态存储。
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// Load 读取状态文件；若文件不存在则返回空状态。
func (s *FileStore) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{
				ContextTokens: make(map[string]string),
			}, nil
		}
		return State{}, err
	}

	var state State
	if err = json.Unmarshal(data, &state); err != nil {
		return State{}, err
	}

	if state.ContextTokens == nil {
		state.ContextTokens = make(map[string]string)
	}

	return state, nil
}

// Save 原子写入当前状态，避免进程中断导致半写文件。
func (s *FileStore) Save(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if state.ContextTokens == nil {
		state.ContextTokens = make(map[string]string)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	tempPath := s.path + ".tmp"
	if err = os.WriteFile(tempPath, data, 0o600); err != nil {
		return err
	}

	if err = os.Rename(tempPath, s.path); err == nil {
		return nil
	}

	// Windows 上已有目标文件时 Rename 可能失败，这里退化为先删后改名。
	if removeErr := os.Remove(s.path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return removeErr
	}

	return os.Rename(tempPath, s.path)
}
