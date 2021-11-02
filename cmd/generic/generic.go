package main

import (
  "fmt"
  "log"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
  "google.golang.org/protobuf/types/known/anypb"
  "google.golang.org/protobuf/types/dynamicpb"

  _ "github.com/Snowflake-Labs/sansshell/services/localfile"
  _ "github.com/Snowflake-Labs/sansshell/services/exec"
)


func main() {
	// let's introspect our proto registry
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		sd := fd.Services()
		if sd.Len() == 0 {
		}
		for i := 0; i < sd.Len(); i++ {
			svc := sd.Get(i)
			md := svc.Methods()
			if md.Len() == 0 {
			}
			for j := 0; j < md.Len(); j++ {
				meth := md.Get(j)
        fmt.Println("Method:", fmt.Sprintf("/%s/%s", svc.FullName(), meth.Name()))
        fmt.Println("Input:", meth.Input().FullName())
        fmt.Println("Output:", meth.Output().FullName())

        in := dynamicpb.NewMessage(meth.Input())
        packedIn, err := anypb.New(in)
        if err != nil  { log.Fatal(err) }
        fmt.Println("input any type url:", packedIn.GetTypeUrl())
			}
		}
		return true
	})
}
