package main

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	protogen.Options{}.Run(func(plugin *protogen.Plugin) error {
		plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		for _, file := range plugin.Files {
			if !file.Generate {
				continue
			}
			generate(plugin, file)
		}
		return nil
	})
}

const (
	contextPackage   = protogen.GoImportPath("context")
	grpcPackage      = protogen.GoImportPath("google.golang.org/grpc")
	grpcProxyPackage = protogen.GoImportPath("github.com/Snowflake-Labs/sansshell/proxy/proxy")
)

func generate(plugin *protogen.Plugin, file *protogen.File) {
	if len(file.Services) == 0 {
		return
	}

	// Create the output file and add the header + includes.
	fn := file.GeneratedFilenamePrefix + "_grpcproxy.pb.go"
	g := plugin.NewGeneratedFile(fn, file.GoImportPath)
	g.P("// Auto generated code by protoc-gen-go-grpcproxy")
	g.P("// DO NOT EDIT")
	g.P()
	g.P("// Adds OneMany versions of RPC methods for use by proxy clients")
	g.P()
	g.P("package ", file.GoPackageName)
	g.P()
	g.P("import (")
	g.P(`"errors"`)
	g.P(`"fmt"`)
	g.P()
	g.P(")")
	g.P()

	for _, service := range file.Services {
		// Since we're adding additional methods on top of those defined we can skip
		// if the service has no methods.
		if len(service.Methods) == 0 {
			continue
		}

		// Need the original names plus our Proxy added ones.
		interfaceName := service.GoName + "Client"
		interfaceNameProxy := interfaceName + "Proxy"
		// Need the internal name of the struct.
		goName := strings.ToLower(interfaceName[:1]) + interfaceName[1:]
		clientStruct := goName
		clientStructProxy := clientStruct + "Proxy"

		// Client interface
		//
		// Have to do this as an embed as we can't just extend the original
		// without replicating the entire grpc plugin and our additions.
		// So we create a new one ending in Proxy which embeds the original
		// and adds our OneMany methods.
		g.P("// ", interfaceNameProxy, " is the superset of ", interfaceName, " which additionally includes the OneMany proxy methods")
		g.P("type ", interfaceNameProxy, " interface {")
		g.P(interfaceName)
		for _, method := range service.Methods {
			methodSignature(false, "", g, method, false)
		}
		g.P("}")
		g.P()
		g.P("// Embed the original client inside of this so we get the other generated methods automatically.")
		g.P("type ", clientStructProxy, " struct {")
		g.P("*", clientStruct)
		g.P("}")
		g.P()

		// Now add a NewFooClientProxy which is equiv to NewFooClient except the
		// object it hands back also had FooOneMany methods. This allows us to use
		// this regardless of using a proxy or not since it also implements Foo methods
		// via embedding and taking any ClientConnInterface (so proxy or the grpc one).
		g.P("// New", interfaceNameProxy, " creates a ", interfaceNameProxy, " for use in proxied connections.")
		g.P("// NOTE: This takes a ProxyConn instead of a generic ClientConnInterface as the methods here are only valid in ProxyConn contexts.")
		g.P("func New", interfaceNameProxy, "(cc *", g.QualifiedGoIdent(grpcProxyPackage.Ident("ProxyConn")), ") ", interfaceNameProxy, " {")
		g.P("return &", clientStructProxy, "{New", interfaceName, "(cc).(*", clientStruct, ")}")
		g.P("}")
		g.P()

		// For each method we have to create the typed response struct
		// (which comes over the channel) and then generate the OneMany methods.
		for _, method := range service.Methods {
			g.P("type ", method.GoName, "ManyResponse struct {")
			g.P("Target string")
			g.P("Resp *", g.QualifiedGoIdent(method.Output.GoIdent))
			g.P("Error error")
			g.P("}")
			g.P()

			methodSignature(true, clientStructProxy, g, method, true)
			unary := !method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer()
			if unary {
				// Unary is simple since we send one thing and just loop over a channel waiting
				// for replies. The only annoyance is type converting from Any in the InvokeMany
				// to the typed response callers expect.
				g.P("conn := c.cc.(*", g.QualifiedGoIdent(grpcProxyPackage.Ident("ProxyConn")), ")")
				g.P("// If this is a single case we can just use Invoke and marshall it onto the channel once and be done.")
				g.P("if conn.NumTargets() == 1 {")
				g.P("out := &", method.GoName, "ManyResponse{")
				g.P("Target: conn.Targets[0],")
				g.P("Resp: &", g.QualifiedGoIdent(method.Output.GoIdent), "{},")
				g.P("}")
				g.P("err := conn.Invoke(ctx, \"/", service.Desc.FullName(), "/", method.Desc.Name(), "\", in, out.Resp, opts...)")
				g.P("if err != nil {")
				g.P("out.Error = err")
				g.P("}")
				g.P("ret := make(chan *", method.GoName, "ManyResponse)")
				g.P("go func() {")
				g.P("// Send and close.")
				g.P("ret <- out")
				g.P("close(ret)")
				g.P("}()")
				g.P("return ret, nil")
				g.P("}")
				g.P("manyRet, err := conn.InvokeOneMany(ctx, \"/", service.Desc.FullName(), "/", method.Desc.Name(), "\", in, opts...)")
				g.P("if err != nil {")
				g.P("return nil, err")
				g.P("}")
				g.P("ret := make(chan *", method.GoName, "ManyResponse)")
				g.P("// A goroutine to retrive untyped responses and convert them to typed ones.")
				g.P("go func() {")
				g.P("for {")
				g.P("typedResp := &", method.GoName, "ManyResponse{")
				g.P("Resp: &", g.QualifiedGoIdent(method.Output.GoIdent), "{},")
				g.P("}")
				g.P()
				g.P("resp, ok := <-manyRet")
				g.P("if !ok {")
				g.P("// All done so we can shut down.")
				g.P("close(ret)")
				g.P("return")
				g.P("}")
				g.P("typedResp.Target = resp.Target")
				g.P("typedResp.Error = resp.Error")
				g.P("if resp.Error == nil {")
				g.P("if err := resp.Resp.UnmarshalTo(typedResp.Resp); err != nil {")
				g.P(`typedResp.Error = fmt.Errorf("can't decode any response - %v. Original Error - %v", err, resp.Error)`)
				g.P("}")
				g.P("}")
				g.P("ret <- typedResp")
				g.P("}")
				g.P("}()")
				g.P()
				g.P("return ret, nil")
				g.P("}")
			} else {
				g.P(`return nil, errors.New("not implemented")`)
				g.P("}")
			}
			g.P()
		}
	}
}

// methodSignature generates the function signature for a OneMany method in both interface form
// and when actually declaring the function (by adding the trailing open brace). It's a little
// clunky but g.P() always adds a newline.
func methodSignature(genFunc bool, structName string, g *protogen.GeneratedFile, method *protogen.Method, addBrace bool) {
	prefix := ""
	if genFunc {
		prefix = fmt.Sprintf("// %sOneMany provides the same API as %s but sends the same request to N destinations at once.\n", method.GoName, method.GoName)
		prefix += "// N can be a single destination.\n"
		prefix += "//\n// NOTE: The returned channel must be read until it closes in order to avoid leaking goroutines.\n"
		prefix += "func (c *" + structName + ") "
	}
	sig := fmt.Sprintf("%s %sOneMany(ctx %s, ", prefix, method.GoName, g.QualifiedGoIdent(contextPackage.Ident("Context")))
	unary := !method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer()
	serverOnly := !method.Desc.IsStreamingClient() && method.Desc.IsStreamingServer()
	if unary || serverOnly {
		sig = fmt.Sprintf("%s in *%s, ", sig, g.QualifiedGoIdent(method.Input.GoIdent))
	}
	brace := ""
	if addBrace {
		brace = " {"
	}
	g.P(sig, "opts ...", g.QualifiedGoIdent(grpcPackage.Ident("CallOption")), ") (<-chan *", method.GoName, "ManyResponse, error)", brace)
}
