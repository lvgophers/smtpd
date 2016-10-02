package config

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

var hostlist = []byte(`example.com
example.net`)
var defaulthost = []byte(`example.org`)

func TestConfig(t *testing.T) {
	td, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(td)
	if err = ioutil.WriteFile(filepath.Join(td, "rcpthosts"), hostlist, 0777); err != nil {
		t.Fatal(err)
	}
	if err = ioutil.WriteFile(filepath.Join(td, "defaulthost"), defaulthost, 0777); err != nil {
		t.Fatal(err)
	}
	conf, err := New(td)
	if err != nil {
		t.Fatal(err)
	}
	d := conf.(*dir)
	if conf.DefaultHost() != string(defaulthost) {
		t.Fatalf("defaulthost wrong: want %s got %s", string(defaulthost), conf.DefaultHost())
	}
	scanner := bufio.NewScanner(bytes.NewReader(hostlist))
	for scanner.Scan() {
		if !conf.Host(scanner.Text()) {
			t.Fatalf("not in rcpthost: %s", scanner.Text())
		}
	}
	if conf.Host("hellno.com") {
		t.Fatal("bogus host OK")
	}
	if !conf.Host(string(defaulthost)) {
		t.Fatalf("default host not added, defaulthost: %s rcpts: %v", d.defaulthost, d.rcpthosts)
	}
	if d.rcpthosts[0] != string(defaulthost) {
		t.Fatal("first rcpthost isn't default")
	}
}
