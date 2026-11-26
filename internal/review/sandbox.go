// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package review

import (
	"fmt"
	"path/filepath"

	"google.golang.org/adk/v2/tool"

	"github.com/go-steer/core-agent/v3/pkg/config"
	"github.com/go-steer/core-agent/v3/pkg/permissions"
	"github.com/go-steer/core-agent/v3/pkg/tools"
)

// ReadOnlyTools returns core-agent's read-only file tools, confined to
// root: read_file, read_many_files, view_file_outline, list_dir, stat,
// glob and grep. Nothing that writes, runs commands or reaches the
// network is registered.
//
// The pull request under review is untrusted input, and a reviewer
// talked into reading a file outside the checkout (credentials, a
// token cache) could quote it into a posted review. So the gate runs in
// "allow" mode with root as the only scope: core-agent denies an
// out-of-scope read without prompting in that mode, where "yolo" would
// allow it. Paths are resolved through symlinks before the check, so a
// symlink in the PR pointing outside root is out of scope too, and
// grep/glob check every file they visit. TestReadOnlyToolsStayInRoot
// pins each of these.
//
// readOnly names further directories the review may read but never
// write, such as a module cache holding the change's dependencies, so a
// claim about a library's API can be checked against its real source.
func ReadOnlyTools(root string, readOnly ...string) ([]tool.Tool, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("review: workspace root: %w", err)
	}
	if abs, err = filepath.EvalSymlinks(abs); err != nil {
		return nil, fmt.Errorf("review: workspace root: %w", err)
	}
	cfg := config.DefaultConfig()
	cfg.Permissions.Mode = config.PermissionModeAllow
	for _, dir := range readOnly {
		d, err := filepath.Abs(dir)
		if err != nil {
			return nil, fmt.Errorf("review: read-only root: %w", err)
		}
		if d, err = filepath.EvalSymlinks(d); err != nil {
			return nil, fmt.Errorf("review: read-only root: %w", err)
		}
		cfg.PathScope.AllowPaths = append(cfg.PathScope.AllowPaths, config.PathScopeAllowEntry{Path: d + "/...", Access: "r"})
	}
	gate, err := permissions.FromConfig(cfg, abs, "", nil)
	if err != nil {
		return nil, fmt.Errorf("review: permission gate: %w", err)
	}
	reg, err := tools.Build(cfg, gate, "", tools.BuiltinTools{
		ReadFile:        true,
		ReadManyFiles:   true,
		ViewFileOutline: true,
		ListDir:         true,
		Stat:            true,
		Glob:            true,
		Grep:            true,
	})
	if err != nil {
		return nil, fmt.Errorf("review: tools: %w", err)
	}
	return reg.Tools, nil
}
