package cleanup

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestCleanupFail(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	timeoutCtx, timeoutCancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer timeoutCancel()
	err = Wait(timeoutCtx, u.Host, 100*time.Millisecond, 3)
	require.EqualError(t, err, "context deadline exceeded")
}

func TestCleanupSucceed(t *testing.T) {
	t.Parallel()

	listenCfg := &net.ListenConfig{}
	listener, err := listenCfg.Listen(t.Context(), "tcp", ":")
	require.NoError(t, err)
	addr := listener.Addr().String()
	err = listener.Close()
	require.NoError(t, err)
	timeoutCtx, timeoutCancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer timeoutCancel()
	g, gCtx := errgroup.WithContext(timeoutCtx)
	g.Go(func() error {
		err := Run(gCtx, addr, t.TempDir())
		if err != nil {
			return err
		}
		return nil
	})
	g.Go(func() error {
		err := Wait(gCtx, addr, 100*time.Microsecond, 3)
		if err != nil {
			return err
		}
		return nil
	})

	err = g.Wait()
	require.NoError(t, err)
}

func TestProbeEndpoints(t *testing.T) {
    t.Parallel()

    handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
        if req.Method != http.MethodGet || req.URL.Path != "/readyz" {
            rw.WriteHeader(http.StatusNotFound)
            return
        }
        rw.WriteHeader(http.StatusOK)
    })

    ts := httptest.NewServer(handler)
    t.Cleanup(func() { ts.Close() })

    // GET /readyz -> 200
    resp, err := ts.Client().Get(ts.URL + "/readyz")
    require.NoError(t, err)
    require.Equal(t, http.StatusOK, resp.StatusCode)

    // GET / -> 404
    resp, err = ts.Client().Get(ts.URL + "/")
    require.NoError(t, err)
    require.Equal(t, http.StatusNotFound, resp.StatusCode)

    // POST /readyz -> 404
    req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/readyz", nil)
    require.NoError(t, err)
    resp, err = ts.Client().Do(req)
    require.NoError(t, err)
    require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
