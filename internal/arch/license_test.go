// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package arch

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// licenseHeader opens every Go file of ours (NOTICE, LICENSE).
const licenseHeader = "// SPDX-License-Identifier: GPL-3.0-only\n// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva\n\n"

// licenseExempt are trees under another license: the MIT port keeps its
// own notice (THIRD_PARTY_NOTICES.md).
var licenseExempt = []string{"pkg/spritenormals/"}

// TestGoFilesCarryTheLicenseHeader: every Go file under cmd, internal and
// pkg starts with the GPL-3.0 SPDX header; python3 scripts/spdx_headers.py
// adds it.
func TestGoFilesCarryTheLicenseHeader(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	var missing []string
	for _, top := range []string{"cmd", "internal", "pkg"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if d.Name() == "testdata" || exempt(rel+"/") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !bytes.HasPrefix(src, []byte(licenseHeader)) {
				missing = append(missing, rel)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("Go files without the license header (python3 scripts/spdx_headers.py adds it):\n%s", strings.Join(missing, "\n"))
	}
}

func exempt(rel string) bool {
	for _, e := range licenseExempt {
		if strings.HasPrefix(rel, e) {
			return true
		}
	}
	return false
}
