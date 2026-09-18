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
	"os"
	"path/filepath"
	"testing"

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
