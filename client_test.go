package tempts_test

import (
	"testing"

	"github.com/vikstrous/tempts"
	"go.temporal.io/sdk/client"
)

// mockSDKClient satisfies client.Client without connecting to a real server.
type mockSDKClient struct {
	client.Client
}

func TestNewFromSDK_NilClient(t *testing.T) {
	_, err := tempts.NewFromSDK(nil, "default")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestNewFromSDK_ValidClient(t *testing.T) {
	c, err := tempts.NewFromSDK(mockSDKClient{}, "test-ns")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewFromSDK_EmptyNamespace(t *testing.T) {
	c, err := tempts.NewFromSDK(mockSDKClient{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
