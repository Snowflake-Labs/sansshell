package mpahooks

import (
	"context"
	"testing"

	"github.com/Snowflake-Labs/sansshell/services/mpa"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestRegisterPayloadProvider(t *testing.T) {
	t.Cleanup(func() { RegisterPayloadProvider(nil) })

	var called bool
	RegisterPayloadProvider(func(ctx context.Context, in PayloadProviderInput) (*anypb.Any, error) {
		called = true
		if in.Method != "/Exec.Exec/Run" {
			t.Fatalf("method = %q", in.Method)
		}
		return anypb.New(&mpa.Action{Method: "marker"})
	})

	payload, err := buildCustomPayload(context.Background(), "/Exec.Exec/Run", &emptypb.Empty{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected provider to be called")
	}
	if payload == nil {
		t.Fatal("expected non-nil payload")
	}
}
