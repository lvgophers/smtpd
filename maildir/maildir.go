package maildir

import (
	"fmt"
	"os"
	"path/filepath"
)

// START OMIT

// Interface is the interface to a maildir.
type Interface interface {
	NewDir() string
	TmpDir() string
}

// END OMIT

type maildir struct {
	basedir string
	newdir  string
	tmpdir  string
}

func (m *maildir) NewDir() string {
	return m.newdir
}

func (m *maildir) TmpDir() string {
	return m.tmpdir
}

var dirnames = []string{"tmp", "new", "cur"}

func (m *maildir) chkdir(name string) (err error) {
	name = filepath.Join(m.basedir, name)
	fi, err := os.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return os.Mkdir(name, 0777)
		}
		return
	}
	if !fi.IsDir() {
		return fmt.Errorf("not a directory: %s", name)
	}
	return
}

// New returns a maildir interface, creating required subdirectories
// if needed.
func New(dir string) (i Interface, err error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}
	md := &maildir{basedir: dir}
	for _, n := range dirnames {
		if err = md.chkdir(n); err != nil {
			return
		}
	}
	md.newdir = filepath.Join(dir, "new")
	md.tmpdir = filepath.Join(dir, "tmp")
	return md, nil
}
