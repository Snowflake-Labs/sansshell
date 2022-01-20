//go:build ignore

package sansshell

//go:generate go build -o bin/proxy-server ./cmd/proxy-server
//go:generate go build -o bin/sanssh ./cmd/sanssh
//go:generate go build -o bin/sansshell-server ./cmd/sansshell-server
