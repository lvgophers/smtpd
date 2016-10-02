package blackbox_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/j7b/smtpd/config"
	"github.com/j7b/smtpd/maildir"
	"github.com/j7b/smtpd/server"
)

var hostlist = []byte(`example.com
example.net`)
var defaulthost = []byte(`example.org`)
var tempdirs = make(chan string, 1024)

func randbytes() []byte {
	rand.Seed(time.Now().UnixNano())
	buf := make([]byte, 64)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return buf
}

func email(to, from string) []byte {
	return []byte(fmt.Sprintf(`Message-ID: <%x>
    Subject: Hi
    To: <%s>
    From: <%s>
    Date: %s

    Hi`, randbytes(), to, from, time.Now().Format(time.RFC1123Z)))
}

func TestBlackBox(t *testing.T) {
	defer func() {
		for dir := range tempdirs {
			os.RemoveAll(dir)
		}
	}()
	defer close(tempdirs)
	var conf config.Interface
	var mdir maildir.Interface
	var serveraddr string
	next := t.Run("Config", func(t *testing.T) {
		td, err := ioutil.TempDir("", "")
		if err != nil {
			t.Fatal(err)
		}
		tempdirs <- td
		if err = ioutil.WriteFile(filepath.Join(td, "rcpthosts"), hostlist, 0777); err != nil {
			t.Fatal(err)
		}
		if err = ioutil.WriteFile(filepath.Join(td, "defaulthost"), defaulthost, 0777); err != nil {
			t.Fatal(err)
		}
		conf, err = config.New(td)
		if err != nil {
			t.Fatal(err)
		}
	})
	if next {
		next = t.Run("Maildir", func(t *testing.T) {
			td, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			tempdirs <- td
			mdir, err = maildir.New(td)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	if next {
		next = t.Run("Server", func(t *testing.T) {
			l, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			serveraddr = l.Addr().String()
			go func() {
				if err := server.Serve(conf, mdir, l); err != nil {
					t.Fatal("server Serve ", err)
				}
			}()
		})
	}
	if next {
		next = t.Run("Client", func(t *testing.T) {
			client, err := smtp.Dial(serveraddr)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			to := "nobody@example.com"
			from := "nobody@nowhere.com"
			if err = client.Mail(from); err != nil {
				t.Fatal(err)
			}
			if err = client.Rcpt(to); err != nil {
				t.Fatal(err)
			}
			if err = client.Rcpt("nobody@lolno.com"); err == nil {
				t.Fatal("expected err for recip not in rcpthosts")
			}
			wc, err := client.Data()
			if err != nil {
				t.Fatal(err)
			}
			mail := email(to, from)
			if _, err = io.Copy(wc, bytes.NewReader(mail)); err != nil {
				t.Fatal(err)
			}
			if err = wc.Close(); err != nil {
				t.Fatal(err)
			}
			if err = client.Quit(); err != nil {
				t.Fatal(err)
			}
			t.Run("Verification", func(t *testing.T) {
				f, err := os.Open(mdir.NewDir())
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				names, err := f.Readdirnames(-1)
				if err != nil {
					t.Fatal(err)
				}
				if len(names) != 1 {
					t.Fatalf("unexpected names in maildir new: %v", names)
				}
				b, err := ioutil.ReadFile(filepath.Join(f.Name(), names[0]))
				if err != nil {
					t.Fatal(err)
				}
				if c := bytes.Compare(bytes.TrimSpace(mail), bytes.TrimSpace(b)); c != 0 {
					t.Logf(`b: "%s"`, string(b))
					t.Logf(`mail: "%s"`, string(mail))
					t.Fatalf("unexpected compare mail to file, got %v want 0", c)
				}
			})
		})
	}
}
