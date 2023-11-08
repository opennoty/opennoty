package testhelper

import (
	"github.com/spf13/afero"
	"io"
	"os"
	"strings"
	"time"
)

// fixedOsFs is a Fs implementation that uses functions provided by the os package.
//
// For details in any method, check the documentation of the os package
// (http://golang.org/pkg/os/).
type fixedOsFs struct{}

func NewOsFs() afero.Fs {
	return &fixedOsFs{}
}

func (fixedOsFs) Name() string { return "fixedOsFs" }

func (fixedOsFs) Create(name string) (afero.File, error) {
	f, e := os.Create(name)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (fixedOsFs) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

func (fixedOsFs) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (fixedOsFs) Open(name string) (afero.File, error) {
	f, e := os.Open(name)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (fixedOsFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	f, e := os.OpenFile(name, flag, perm)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (fixedOsFs) Remove(name string) error {
	return os.Remove(name)
}

func (fixedOsFs) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	oldStat, err := in.Stat()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	err = out.Sync()
	if err != nil {
		return err
	}
	out.Chmod(oldStat.Mode())
	return nil
}

func (fixedOsFs) Rename(oldname, newname string) error {
	if err := os.Rename(oldname, newname); err != nil {
		if strings.Contains(err.Error(), "invalid cross-device link") {
			err = copyFile(oldname, newname)
			if err == nil {
				os.Remove(oldname)
			}
			return err
		}
		return err
	}
	return nil
}

func (fixedOsFs) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (fixedOsFs) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}

func (fixedOsFs) Chown(name string, uid, gid int) error {
	return os.Chown(name, uid, gid)
}

func (fixedOsFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return os.Chtimes(name, atime, mtime)
}

func (fixedOsFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	fi, err := os.Lstat(name)
	return fi, true, err
}

func (fixedOsFs) SymlinkIfPossible(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (fixedOsFs) ReadlinkIfPossible(name string) (string, error) {
	return os.Readlink(name)
}
