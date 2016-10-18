package session

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lvgophers/smtpd/types"
)

const email = `Subject: hai

Hai!
`

type testconfig struct{}

func (t *testconfig) Host(name string) bool {
	return true
}
func (t *testconfig) Reload() (err error) {
	return nil
}
func (t *testconfig) DefaultHost() string {
	return "none"
}
func (t *testconfig) Timeout() time.Duration {
	return 3 * time.Second
}
func (t *testconfig) MaxRcpt() int {
	return 1
}
func (t *testconfig) MaxSize() int64 {
	return 1024 * 1024
}

type testmaildir struct {
	basedir string
}

func td() *testmaildir {
	check := func(e error) {
		if e != nil {
			panic(e)
		}
	}
	dir, err := ioutil.TempDir("", "")
	check(err)
	check(os.Mkdir(filepath.Join(dir, "tmp"), 0777))
	check(os.Mkdir(filepath.Join(dir, "new"), 0777))
	return &testmaildir{basedir: dir}
}

func (t *testmaildir) NewDir() string {
	return filepath.Join(t.basedir, "new")
}

func (t *testmaildir) TmpDir() string {
	return filepath.Join(t.basedir, "tmp")
}

func TestSession(t *testing.T) {
	l, err := net.Listen("tcp4", ":0")
	if err != nil {
		t.Fatal(err)
	}
	var client *smtp.Client
	clientok := make(chan struct{}, 0)
	go func() {
		defer close(clientok)
		var err error
		client, err = smtp.Dial(l.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
	}()
	c, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	tp := textproto.NewConn(c)
	tp.PrintfLine("220 hi")
	<-clientok
	if client == nil {
		t.Fatal("Unexpectedly nil client")
	}
	maildir := td()
	ses := New(&types.NetConn{C: c, Conn: tp}, &testconfig{}, maildir)
	go ses.Start()
	err = client.Hello("hai")
	if err != nil {
		t.Fatal(err)
	}
	err = client.Mail("nobody@nowhere.com")
	if err != nil {
		t.Fatal(err)
	}
	err = client.Reset()
	if err != nil {
		t.Fatal(err)
	}
	err = client.Verify("nobody@nowhere.com")
	if err == nil {
		t.Fatal("Expected verify to error")
	}
	t.Log("verify error ok:", err)
	err = client.Mail("nobody@nowhere.com")
	if err != nil {
		t.Fatal(err)
	}
	err = client.Rcpt("somebody@somewhere.com")
	if err != nil {
		t.Fatal(err)
	}
	err = client.Rcpt("somebodyelse@somewhere.com")
	if err == nil {
		t.Fatal("expected max rcpt 1")
	}
	t.Log("rcpt error ok: ", err)
	wc, err := client.Data()
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(wc, bytes.NewReader([]byte(email)))
	if err != nil {
		t.Fatal(err)
	}
	err = wc.Close()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(maildir.NewDir())
	if err != nil {
		t.Fatal(err)
	}
	names, err := f.Readdirnames(-1)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 {
		t.Fatal("unexpected len names: ", len(names))
	}
	b, err := ioutil.ReadFile(filepath.Join(maildir.NewDir(), names[0]))
	if err != nil {
		t.Fatal(err)
	}
	if c := bytes.Compare(b, []byte(email)); c != 0 {
		t.Fatalf("email and %s unexpectedly different: %v", names[0], c)
	}
	err = client.Quit()
	if err != nil {
		t.Fatal(err)
	}
}
