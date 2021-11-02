package server

import (
	"testing"

	"google.golang.org/protobuf/proto"

	testpb "github.com/Snowflake-Labs/sansshell/proxy/testdata"
)

func TestLoadServiceMap(t *testing.T) {
	serviceMap := LoadGlobalServiceMap()
	if len(serviceMap) == 0 {
		t.Fatal("LoadGlobalServiceMap() result is empty, want non-empty")
	}
	for _, name := range []string{
		"/Testdata.TestServiceWithoutMethods/Foo",
		"/Testdata.TestService/NoSuchMethod",
		"/Testdata.NoSuchService/NoSuchMethod",
	} {
		_, ok := serviceMap[name]
		if ok {
			t.Errorf("serviceMap entry for %s was present, want missing", name)
		}
	}
	for _, tc := range []struct {
		method        string
		input         proto.Message
		output        proto.Message
		clientStreams bool
		serverStreams bool
	}{
		{
			method:        "/Testdata.TestService/TestUnary",
			input:         &testpb.TestRequest{},
			output:        &testpb.TestResponse{},
			clientStreams: false,
			serverStreams: false,
		},
		{
			method:        "/Testdata.TestService/TestServerStream",
			input:         &testpb.TestRequest{},
			output:        &testpb.TestResponse{},
			clientStreams: false,
			serverStreams: true,
		},
		{
			method:        "/Testdata.TestService/TestClientStream",
			input:         &testpb.TestRequest{},
			output:        &testpb.TestResponse{},
			clientStreams: true,
			serverStreams: false,
		},
		{
			method:        "/Testdata.TestService/TestBidiStream",
			input:         &testpb.TestRequest{},
			output:        &testpb.TestResponse{},
			clientStreams: true,
			serverStreams: true,
		},
	} {
		m, ok := serviceMap[tc.method]
		if !ok {
			t.Fatalf("method '%s' was was not found in service map, expected present", tc.method)
		}
		if tc.method != m.FullName() {
			t.Errorf("%s fullname was %s, want %s", tc.method, m.FullName(), tc.method)
		}
		if tc.clientStreams != m.ClientStreams() {
			t.Errorf("%s client streaming was %v, want %v", tc.method, m.ClientStreams(), tc.clientStreams)
		}
		if tc.serverStreams != m.ServerStreams() {
			t.Errorf("%s server streaming was %v, want %v", tc.method, m.ServerStreams(), tc.serverStreams)
		}
		req := m.NewRequest()
		if !proto.Equal(tc.input, req) {
			t.Errorf("%s request message was %v, want %v", tc.method, req, tc.input)
		}
		rep := m.NewReply()
		if !proto.Equal(tc.output, rep) {
			t.Errorf("%s reply message was %v, want %$v", tc.method, rep, tc.output)
		}
	}
}
