package mtail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/google/mtail/internal/metrics"
	"github.com/google/mtail/internal/watcher"
	"github.com/spf13/afero"
)

// TestTempDir creates a temporary directory for use during tests. It returns
// the pathname, and a cleanup function.
func TestTempDir(t *testing.T) (string, func()) {
	t.Helper()
	name, err := ioutil.TempDir("", "mtail-test")
	if err != nil {
		t.Fatal(err)
	}
	return name, func() {
		if err := os.RemoveAll(name); err != nil {
			t.Fatalf("os.RemoveAll(%s): %s", name, err)
		}
	}
}

// TestMakeServer makes a new Server for use in tests, but does not start
// the server.  It returns the server, or any errors the new server creates.
func TestMakeServer(t *testing.T, options ...func(*Server) error) (*Server, error) {
	t.Helper()
	w, err := watcher.NewLogWatcher(0, true)
	if err != nil {
		t.Fatal(err)
	}

	return New(metrics.NewStore(), w, &afero.OsFs{}, options...)
}

// TestStartServer creates a new Server and starts it running.  It
// returns the server, and a cleanup function.
func TestStartServer(t *testing.T, options ...func(*Server) error) (*Server, func()) {
	t.Helper()
	options = append(options, BindAddress("", "0"))

	m, err := TestMakeServer(t, options...)
	if err != nil {
		t.Fatal(err)
	}

	errc := make(chan error, 1)
	go func() {
		err := m.Run()
		errc <- err
	}()

	glog.Infof("check that server is listening")
	count := 0
	for _, err := net.DialTimeout("tcp", m.Addr(), 10*time.Millisecond); err != nil && count < 10; count++ {
		glog.Infof("err: %s, retrying to dial %s", err, m.Addr())
		time.Sleep(100 * time.Millisecond)
	}
	if count >= 10 {
		t.Fatal("server wasn't listening after 10 attempts")
	}

	return m, func() {
		m.Close()
		err := <-errc
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestGetMetric fetches the expvar metrics from the Server at addr, and
// returns the value of one named name.  Callers are responsible for type
// assertions on the returned value.
func TestGetMetric(t *testing.T, addr, name string) interface{} {
	t.Helper()
	uri := fmt.Sprintf("http://%s/debug/vars", addr)
	resp, err := http.Get(uri)
	if err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	var r map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r[name]
}
