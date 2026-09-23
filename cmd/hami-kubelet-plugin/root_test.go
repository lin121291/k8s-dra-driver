/*
 * Copyright 2026 The HAMi Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDriverRootSearchPaths(t *testing.T) {
	tests := []struct {
		name    string
		lookup  func(root) (string, error)
		file    string
		dirs    []string
		symlink bool
	}{
		{
			name: "local library", lookup: root.getDriverLibraryPath,
			file: "libnvidia-ml.so.1", dirs: []string{"usr/local/lib"},
		},
		{
			name: "local lib64 library", lookup: root.getDriverLibraryPath,
			file: "libnvidia-ml.so.1", dirs: []string{"usr/local/lib64"},
		},
		{
			name: "local binary", lookup: root.getNvidiaSMIPath,
			file: "nvidia-smi", dirs: []string{"usr/local/bin"},
		},
		{
			name: "local binary symlink", lookup: root.getNvidiaSMIPath,
			file: "nvidia-smi", dirs: []string{"usr/local/bin"}, symlink: true,
		},
		{
			name: "existing library path takes precedence", lookup: root.getDriverLibraryPath,
			file: "libnvidia-ml.so.1", dirs: []string{"usr/lib64", "usr/local/lib", "usr/local/lib64"},
		},
		{
			name: "existing binary path takes precedence", lookup: root.getNvidiaSMIPath,
			file: "nvidia-smi", dirs: []string{"usr/bin", "usr/local/bin"},
		},
		{
			name: "local library symlink", lookup: root.getDriverLibraryPath,
			file: "libnvidia-ml.so.1", dirs: []string{"usr/local/lib"}, symlink: true,
		},
		{
			name: "missing library", lookup: root.getDriverLibraryPath,
		},
		{
			name: "missing binary", lookup: root.getNvidiaSMIPath,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driverRoot := t.TempDir()
			filename := tt.file
			if tt.symlink {
				filename += ".target"
			}
			for _, dir := range tt.dirs {
				path := filepath.Join(driverRoot, dir)
				require.NoError(t, os.MkdirAll(path, 0755))
				require.NoError(t, os.WriteFile(filepath.Join(path, filename), nil, 0644))
				if tt.symlink {
					require.NoError(t, os.Symlink(filename, filepath.Join(path, tt.file)))
				}
			}

			got, err := tt.lookup(root(driverRoot))
			if len(tt.dirs) == 0 {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, filepath.Join(driverRoot, tt.dirs[0], filename), got)
		})
	}
}

// TestPrestartNvidiaSMISearch exercises the script's lookup without starting its
// validation loop, which requires a mounted driver root and a working GPU.
func TestPrestartNvidiaSMISearch(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "hack", "kubelet-plugin-prestart.sh"))
	require.NoError(t, err)
	lookup := regexp.MustCompile(`(?ms)^\s*NV_PATH=\$\((.*?)^\s*\)`).FindSubmatch(script)
	require.Len(t, lookup, 2, "prestart script must contain the NV_PATH lookup")
	command := strings.ReplaceAll(string(lookup[1]), "/driver-root", `"$TEST_DRIVER_ROOT"`)

	tests := []struct {
		name  string
		files []string
		links map[string]string
		want  string
	}{
		{
			name:  "local binary",
			files: []string{"usr/local/bin/nvidia-smi"},
			want:  "usr/local/bin/nvidia-smi",
		},
		{
			name:  "local binary symlink",
			files: []string{"usr/local/bin/nvidia-smi.real"},
			links: map[string]string{"usr/local/bin/nvidia-smi": "nvidia-smi.real"},
			want:  "usr/local/bin/nvidia-smi",
		},
		{
			name:  "existing binary path takes precedence",
			files: []string{"usr/bin/nvidia-smi", "usr/local/bin/nvidia-smi.real"},
			links: map[string]string{"usr/local/bin/nvidia-smi": "nvidia-smi.real"},
			want:  "usr/bin/nvidia-smi",
		},
		{
			name:  "existing binary symlink takes precedence",
			files: []string{"usr/bin/nvidia-smi.real", "usr/local/bin/nvidia-smi"},
			links: map[string]string{"usr/bin/nvidia-smi": "nvidia-smi.real"},
			want:  "usr/bin/nvidia-smi",
		},
		{
			name:  "broken binary symlink",
			links: map[string]string{"usr/local/bin/nvidia-smi": "missing"},
		},
		{
			name: "missing binary",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driverRoot := filepath.Join(t.TempDir(), "driver root")
			for _, file := range tt.files {
				path := filepath.Join(driverRoot, file)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
				require.NoError(t, os.WriteFile(path, nil, 0755))
			}
			for link, target := range tt.links {
				path := filepath.Join(driverRoot, link)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
				require.NoError(t, os.Symlink(target, path))
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-c", command)
			cmd.Env = append(os.Environ(), "TEST_DRIVER_ROOT="+driverRoot)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", output)
			want := ""
			if tt.want != "" {
				want = filepath.Join(driverRoot, tt.want)
			}
			require.Equal(t, want, strings.TrimSpace(string(output)))
		})
	}
}
