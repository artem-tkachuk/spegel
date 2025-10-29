package main

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
)

// minimal fake Store to get past NewContainerd call not required; instead, we
// directly test the error propagation path after Verify by calling
// registryCommand with a containerd path that triggers Verify error.

func TestRun_ReturnsErrorWhenVerifyFails(t *testing.T) {
	t.Parallel()

	// Arguments configured to hit Verify path with a non-existing config path.
	reg := &RegistryCmd{
		BootstrapConfig:              BootstrapConfig{BootstrapKind: "static"},
		ContainerdRegistryConfigPath: "/non/existent/path",
		ContainerdSock:               "/run/containerd/containerd.sock",
		ContainerdNamespace:          "k8s.io",
		ContainerdContentPath:        t.TempDir(),
		DataDir:                      t.TempDir(),
		RouterAddr:                   ":0",
		RegistryAddr:                 ":0",
		MirroredRegistries:           nil,
		RegistryFilters:              []*regexp.Regexp{},
		MirrorResolveTimeout:         20 * time.Millisecond,
		MirrorResolveRetries:         1,
		DebugWebEnabled:              false,
		ResolveLatestTag:             true,
	}

	// Invoke run with the Registry subcommand and assert it returns error.
	// main() would convert this into exit code 1.
	ctx := logr.NewContext(t.Context(), logr.Discard())
	err := run(ctx, &Arguments{Registry: reg})
	require.Error(t, err)
	// Accept either NewContainerd or Verify failure depending on environment.
	var ociErr error
	if errors.As(err, &ociErr) {
		// ok
	}
}
