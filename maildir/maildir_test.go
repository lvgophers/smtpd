package maildir

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	td, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(td)
	maildir, err := New(td)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range dirnames {
		if fi, err := os.Stat(filepath.Join(td, n)); err == nil {
			if !fi.IsDir() {
				t.Fatalf("not a directory: %s", n)
			}
		} else {
			t.Fatal(err)
		}
	}
	if d := maildir.TmpDir(); d != filepath.Join(td, "tmp") {
		t.Fatal("Bad TmpDir: ", d)
	}
	if d := maildir.NewDir(); d != filepath.Join(td, "new") {
		t.Fatal("Bad NewDir: ", d)
	}
}
