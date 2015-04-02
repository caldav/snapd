/*
 * Copyright (C) 2014-2015 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

// Package policy provides helpers for keeping a framework's security policies
// up to date on install/remove.
package policy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	secbase = "/var/lib/snappy"
)

type policyOp uint

const (
	// Install copies the policy files from the framework snap to the
	// right place, with the necessary renaming.
	Install policyOp = iota
	// Remove cleans the policy files up again.
	Remove
)

func (op policyOp) String() string {
	switch op {
	case Remove:
		return "Remove"
	case Install:
		return "Install"
	default:
		return fmt.Sprintf("policyOp(%d)", op)
	}
}

// helper iterates over all the files found with the given glob, making the
// basename (with the given suffix prepended) the target file in the given
// target directory. It then performs op on that target file: either copying
// from the globbed file to the target file, or removing the target file.
// Directories are created as needed. Errors out with any of the things that
// could go wrong with this, including a file found by glob not being a
// regular file.
func helper(op policyOp, glob string, targetDir string, suffix string) (err error) {
	if err = os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("unable to make %v directory: %v", targetDir, err)
	}
	files, err := filepath.Glob(glob)
	if err != nil {
		// filepath.Glob seems to not return errors ever right
		// now. This might be a bug in Go, or it might be by
		// design. Better play safe.
		return fmt.Errorf("unable to glob %v: %v", glob, err)
	}
	for _, file := range files {
		s, err := os.Lstat(file)
		if err != nil {
			return fmt.Errorf("unable to stat %v: %v", file, err)
		}
		if !s.Mode().IsRegular() {
			return fmt.Errorf("unable to do %s for %v: not a regular file", op, file)
		}
		targetFile := filepath.Join(targetDir, suffix+filepath.Base(file))
		switch op {
		case Remove:
			if err = os.Remove(targetFile); err != nil {
				return fmt.Errorf("unable to remove %v: %v", targetFile, err)
			}
		case Install:
			// do the copy
			fin, err := os.Open(file)
			if err != nil {
				return fmt.Errorf("unable to read %v: %v", file, err)
			}
			defer func() {
				if cerr := fin.Close(); cerr != nil && err == nil {
					err = fmt.Errorf("when closing %v: %v", file, cerr)
				}
			}()
			fout, err := os.Create(targetFile)
			if err != nil {
				return fmt.Errorf("unable to create %v: %v", targetFile, err)
			}
			defer func() {
				if cerr := fout.Close(); cerr != nil && err == nil {
					err = fmt.Errorf("when closing %v: %v", targetFile, cerr)
				}
			}()
			if _, err = io.Copy(fout, fin); err != nil {
				return fmt.Errorf("unable to copy %v to %v: %v", file, targetFile, err)
			}
			if err = fout.Sync(); err != nil {
				return fmt.Errorf("when syncing %v: %v", targetFile, err)
			}
		default:
			return fmt.Errorf("unknown operation %s", op)
		}
	}
	return nil
}

// FrameworkOp perform the given operation (either Install or Remove) on the
// given package that's installed in the given path.
func FrameworkOp(op policyOp, pkgName string, instPath string) (err error) {
	pol := filepath.Join(instPath, "meta", "framework-policy")
	for _, i := range []string{"apparmor", "seccomp"} {
		for _, j := range []string{"policygroups", "templates"} {
			if err = helper(op, filepath.Join(pol, i, j, "*"), filepath.Join(secbase, i, j), pkgName+"_"); err != nil {
				return err
			}
		}
	}
	return nil
}
