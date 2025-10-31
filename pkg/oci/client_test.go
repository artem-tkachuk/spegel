package oci

import (
    "bytes"
    "io"
    "net/http"
    "net/http/httptest"
    "net/url"
    "os"
    "path/filepath"
    "testing"

	"cuelabs.dev/go/oci/ociregistry/ocimem"
	"cuelabs.dev/go/oci/ociregistry/ociserver"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spegel-org/spegel/pkg/httpx"
	"github.com/stretchr/testify/require"
)

// roundTripperFunc allows using a function as an http.RoundTripper in tests.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// trackingBody tracks whether Close was called.
type trackingBody struct{ closed *bool }

func (tb trackingBody) Read(p []byte) (int, error) { return 0, io.EOF }

func (tb trackingBody) Close() error { *tb.closed = true; return nil }

func TestClient(t *testing.T) {
	t.Parallel()

	img, err := ParseImage("docker.io/test/image:latest", AllowTagOnly())
	require.NoError(t, err)

	mem := ocimem.New()
	blobs := []ocispec.Descriptor{
		{
			MediaType: ocispec.MediaTypeImageConfig,
			Digest:    digest.Digest("sha256:68b8a989a3e08ddbdb3a0077d35c0d0e59c9ecf23d0634584def8bdbb7d6824f"),
			Size:      529,
		},
		{
			MediaType: ocispec.MediaTypeImageLayerGzip,
			Digest:    digest.Digest("sha256:3caa2469de2a23cbcc209dd0b9d01cd78ff9a0f88741655991d36baede5b0996"),
			Size:      118,
		},
	}
	for _, blob := range blobs {
		f, err := os.Open(filepath.Join("testdata", "blobs", "sha256", blob.Digest.Encoded()))
		require.NoError(t, err)
		_, err = mem.PushBlob(t.Context(), img.Repository, blob, f)
		f.Close()
		require.NoError(t, err)
	}
	manifests := []ocispec.Descriptor{
		{
			MediaType: ocispec.MediaTypeImageManifest,
			Digest:    digest.Digest("sha256:b6d6089ca6c395fd563c2084f5dd7bc56a2f5e6a81413558c5be0083287a77e9"),
		},
	}
	for _, manifest := range manifests {
		b, err := os.ReadFile(filepath.Join("testdata", "blobs", "sha256", manifest.Digest.Encoded()))
		require.NoError(t, err)
		_, err = mem.PushManifest(t.Context(), img.Repository, img.Tag, b, manifest.MediaType)
		require.NoError(t, err)
	}
	reg := ociserver.New(mem, nil)
	srv := httptest.NewServer(reg)
	t.Cleanup(func() {
		srv.Close()
	})

	client := NewClient(srv.Client())
	mirror, err := url.Parse(srv.URL)
	require.NoError(t, err)
	pullResults, err := client.Pull(t.Context(), img, WithPullMirror(mirror))
	require.NoError(t, err)
	require.Len(t, pullResults, 3)

	ref := Reference{
		Registry:   img.Registry,
		Repository: img.Repository,
		Digest:     blobs[0].Digest,
	}
	dist, err := NewDistributionPath(ref, DistributionKindBlob)
	require.NoError(t, err)
	desc, err := client.Head(t.Context(), dist, WithFetchMirror(mirror))
	require.NoError(t, err)
	require.Equal(t, dist.Digest, desc.Digest)
	require.Equal(t, httpx.ContentTypeBinary, desc.MediaType)

	client = NewClient(nil)
	require.NotNil(t, client.httpClient)
}

func TestDescriptorHeader(t *testing.T) {
	t.Parallel()

	header := http.Header{}
	desc := ocispec.Descriptor{
		MediaType: "foo",
		Size:      909,
		Digest:    digest.Digest("sha256:b6d6089ca6c395fd563c2084f5dd7bc56a2f5e6a81413558c5be0083287a77e9"),
	}
	WriteDescriptorToHeader(desc, header)
	require.Equal(t, "foo", header.Get(httpx.HeaderContentType))
	require.Equal(t, "909", header.Get(httpx.HeaderContentLength))
	require.Equal(t, "sha256:b6d6089ca6c395fd563c2084f5dd7bc56a2f5e6a81413558c5be0083287a77e9", header.Get(HeaderDockerDigest))
	headerDesc, err := DescriptorFromHeader(header)
	require.NoError(t, err)
	require.Equal(t, desc, headerDesc)

	tests := []struct {
		name     string
		header   http.Header
		expected string
	}{
		{
			name: "missing content type",
			header: http.Header{
				httpx.HeaderContentLength: {"1"},
				HeaderDockerDigest:        {"sha256:9fccb471b0f2482af80f8bd7b198dfe3afedb16e683fdd30a17423a32be54d10"},
			},
			expected: "content type cannot be empty",
		},
		{
			name: "missing content length",
			header: http.Header{
				httpx.HeaderContentType: {httpx.ContentTypeBinary},
				HeaderDockerDigest:      {"sha256:9fccb471b0f2482af80f8bd7b198dfe3afedb16e683fdd30a17423a32be54d10"},
			},
			expected: "content length cannot be empty",
		},
		{
			name: "non int content length",
			header: http.Header{
				httpx.HeaderContentType:   {httpx.ContentTypeBinary},
				httpx.HeaderContentLength: {"bar"},
				HeaderDockerDigest:        {"sha256:9fccb471b0f2482af80f8bd7b198dfe3afedb16e683fdd30a17423a32be54d10"},
			},
			expected: "strconv.ParseInt: parsing \"bar\": invalid syntax",
		},
		{
			name: "missing digest",
			header: http.Header{
				httpx.HeaderContentType:   {httpx.ContentTypeBinary},
				httpx.HeaderContentLength: {"1"},
			},
			expected: "invalid checksum digest format",
		},
		{
			name: "invalid digest format",
			header: http.Header{
				httpx.HeaderContentType:   {httpx.ContentTypeBinary},
				httpx.HeaderContentLength: {"1"},
				HeaderDockerDigest:        {"foo"},
			},
			expected: "invalid checksum digest format",
		},
		{
			name: "invalid content range unit",
			header: http.Header{
				httpx.HeaderContentType:   {httpx.ContentTypeBinary},
				httpx.HeaderContentLength: {"1"},
				HeaderDockerDigest:        {"sha256:9fccb471b0f2482af80f8bd7b198dfe3afedb16e683fdd30a17423a32be54d10"},
				httpx.HeaderContentRange:  {"foo 1-3/40"},
			},
			expected: "unsupported content range unit foo 1-3/40",
		},
		{
			name: "invalid content range format",
			header: http.Header{
				httpx.HeaderContentType:   {httpx.ContentTypeBinary},
				httpx.HeaderContentLength: {"1"},
				HeaderDockerDigest:        {"sha256:9fccb471b0f2482af80f8bd7b198dfe3afedb16e683fdd30a17423a32be54d10"},
				httpx.HeaderContentRange:  {"bytes 1-3 40"},
			},
			expected: "unexpected content range format bytes 1-3 40",
		},
		{
			name: "undefined size",
			header: http.Header{
				httpx.HeaderContentType:   {httpx.ContentTypeBinary},
				httpx.HeaderContentLength: {"1"},
				HeaderDockerDigest:        {"sha256:9fccb471b0f2482af80f8bd7b198dfe3afedb16e683fdd30a17423a32be54d10"},
				httpx.HeaderContentRange:  {"bytes 1-3/*"},
			},
			expected: "content range expected to specify size bytes 1-3/*",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := DescriptorFromHeader(tt.header)
			require.EqualError(t, err, tt.expected)
		})
	}
}

// Test that on 401 the initial response body is properly drained/closed before retrying auth.
func TestFetch_ClosesBodyOnUnauthorized(t *testing.T) {
    t.Parallel()

    // Track closes on the 401 body.
    closed := false
    tb := trackingBody{closed: &closed}

    // First request returns 401 with WWW-Authenticate; second is token; third is success.
    var stage int
    rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
        stage++
        switch stage {
        case 1:
            return &http.Response{
                StatusCode: http.StatusUnauthorized,
                Header: http.Header{
                    httpx.HeaderWWWAuthenticate: {"Bearer realm=\"https://auth.example/token\""},
                },
                Body: tb,
                Request: req,
            }, nil
        case 2:
            // token fetch
            if req.URL.String() == "https://auth.example/token" {
                return &http.Response{
                    StatusCode: http.StatusOK,
                    Body: io.NopCloser(bytes.NewBufferString("{\"token\":\"abc\"}")),
                    Request: req,
                }, nil
            }
        }
        // final data fetch
        return &http.Response{
            StatusCode: http.StatusOK,
            Header: http.Header{
                httpx.HeaderContentType:   {httpx.ContentTypeBinary},
                httpx.HeaderContentLength: {"1"},
                HeaderDockerDigest:        {"sha256:9fccb471b0f2482af80f8bd7b198dfe3afedb16e683fdd30a17423a32be54d10"},
            },
            Body: io.NopCloser(bytes.NewBuffer([]byte{0})),
            Request: req,
        }, nil
    })
    httpClient := &http.Client{Transport: rt}
    client := NewClient(httpClient)

    img, err := ParseImage("example.com/org/repo:tag", AllowTagOnly())
    require.NoError(t, err)
    dist := img.DistributionPath()
    _, _, err = client.Fetch(t.Context(), http.MethodGet, dist)
    require.NoError(t, err)
    require.True(t, closed, "401 response body should be closed before retry")
}
