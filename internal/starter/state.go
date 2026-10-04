package starter

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

type diskState struct {
	Version int                  `json:"version"`
	Claims  map[string]time.Time `json:"claims"`
	Results []Result             `json:"results"`
}

func readState(path string) (diskState, error) {
	state := diskState{Version: 1, Claims: map[string]time.Time{}, Results: []Result{}}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return state, errors.New("无法读取触发记录，请检查状态文件权限")
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 8*1024*1024+1))
	if err != nil || len(raw) > 8*1024*1024 {
		return state, errors.New("触发记录无法读取或文件过大")
	}
	if json.Unmarshal(raw, &state) != nil || state.Version != 1 || state.Claims == nil {
		return state, errors.New("触发记录损坏，已停止自动触发，避免重复发送")
	}
	for i := range state.Results {
		if state.Results[i].Status == "running" {
			state.Results[i].Status = "uncertain"
			state.Results[i].Message = "上次运行中断，结果不确定；不会自动重复发送"
		}
	}
	return state, nil
}

func writeState(path string, state diskState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.New("无法创建触发记录目录")
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errors.New("无法编码触发记录")
	}
	file, err := os.CreateTemp(dir, ".window-starter-*.tmp")
	if err != nil {
		return errors.New("无法创建触发记录文件")
	}
	temp := file.Name()
	// The temporary file is plugin-owned and can be removed on a failed write.
	defer os.Remove(temp)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return errors.New("无法保存触发记录，已停止发送")
	}
	if err = os.Rename(temp, path); err != nil {
		return errors.New("无法更新触发记录，已停止发送")
	}
	if directory, errOpen := os.Open(dir); errOpen == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}
