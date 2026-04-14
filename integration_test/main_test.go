package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/testsuite"
)

var testClient *tempts.Client

func TestMain(m *testing.M) {
	ctx := context.Background()

	srv, err := testsuite.StartDevServer(ctx, testsuite.DevServerOptions{
		LogLevel: "error",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start dev server: %v\n", err)
		os.Exit(1)
	}

	testClient, err = tempts.NewFromSDK(srv.Client(), "default")
	if err != nil {
		srv.Stop()
		fmt.Fprintf(os.Stderr, "failed to create client: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	srv.Stop()
	os.Exit(code)
}
