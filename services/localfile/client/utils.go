package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/Snowflake-Labs/sansshell/proxy/proxy"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"
)

// ReadRemoteFile is a helper function for reading a single file from a remote host
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func ReadRemoteFile(ctx context.Context, conn *proxy.Conn, path string) ([]byte, error) {
	if len(conn.Targets) != 1 {
		return nil, errors.New("ReadRemoteFile only supports single targets")
	}

	c := pb.NewLocalFileClient(conn)
	stream, err := c.Read(ctx, &pb.ReadActionRequest{
		Request: &pb.ReadActionRequest_File{
			File: &pb.ReadRequest{
				Filename: path,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("can't setup Read client stream: %v", err)
	}

	var ret []byte
	for {
		resp, err := stream.Recv()
		// Stream is done.
		if err == io.EOF {
			break
		}
		// Some other error.
		if err != nil {
			return nil, fmt.Errorf("can't read file %s - %v", path, err)
		}
		ret = append(ret, resp.Contents...)
	}
	return ret, nil
}

// FileConfig defines a configuration defining a remote file.
// This will be used when defining a remote written file such
// as writing a new file or copying one.
type FileConfig struct {
	// Filename is the remote full path to write the file.
	Filename string

	// User is the remote user to chown() the file ownership.
	User string

	// Group is the remote group to chgrp() the file group.
	Group string

	// Perms are the standard unix file permissions for the remote file.
	Perms int

	// If overwrite is true the remote file will be overwritten if it exists,
	// otherwise it's an error to write to an existing file.
	Overwrite bool
}

// WriteRemoteFile is a helper function for writing a single file to a remote host
// using a proxy.Conn.
func WriteRemoteFile(ctx context.Context, conn *proxy.Conn, config *FileConfig, contents []byte) error {
	c := pb.NewLocalFileClientProxy(conn)
	stream, err := c.WriteOneMany(ctx)
	if err != nil {
		return fmt.Errorf("can't setup Write stream - %v", err)
	}

	req := &pb.WriteRequest{
		Request: &pb.WriteRequest_Description{
			Description: &pb.FileWrite{
				Overwrite: config.Overwrite,
				Attrs: &pb.FileAttributes{
					Filename: config.Filename,
					Attributes: []*pb.FileAttribute{
						{
							Value: &pb.FileAttribute_Mode{
								Mode: uint32(config.Perms),
							},
						},
						{
							Value: &pb.FileAttribute_Username{
								Username: config.User,
							},
						},
						{
							Value: &pb.FileAttribute_Group{
								Group: config.Group,
							},
						},
					},
				},
			},
		},
	}

	// Send setup packet
	if err := stream.Send(req); err != nil {
		return fmt.Errorf("can't send setup for writing file %s - %v", config.Filename, err)
	}
	// Send content in chunks
	buf := make([]byte, util.StreamingChunkSize)
	reader := bytes.NewBuffer(contents)
	for {
		n, err := reader.Read(buf)
		if err == io.EOF {
			break
		}

		req := &pb.WriteRequest{
			Request: &pb.WriteRequest_Contents{
				// Only send up to n as the last read is often a short read.
				Contents: buf[:n],
			},
		}
		if err := stream.Send(req); err != nil {
			// Emit this to every error file as it's not specific to a given target.
			return fmt.Errorf("failed to send file contents: %v\n", err)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to close stream: %v\n", err)
	}
	// There are no responses to process but we do need to check for errors.
	for _, r := range resp {
		if r.Error != nil && r.Error != io.EOF {
			return fmt.Errorf("target returned error: %v\n", r.Error)
		}
	}
	return nil
}

// RemoveRemoteFile is a helper function for removing a file on a remote host
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func RemoveRemoteFile(ctx context.Context, conn *proxy.Conn, path string) error {
	if len(conn.Targets) != 1 {
		return errors.New("RemoveRemoteFile only supports single targets")
	}

	c := pb.NewLocalFileClient(conn)
	_, err := c.Rm(ctx, &pb.RmRequest{
		Filename: path,
	})
	if err != nil {
		return fmt.Errorf("remove problem - %v", err)
	}
	return nil
}

// CopyRemoteFile is a helper function for copying a file on a remote host
// using a proxy.Conn. If the conn is defined for >1 targets this will return an error.
func CopyRemoteFile(ctx context.Context, conn *proxy.Conn, source string, destination *FileConfig) error {
	if len(conn.Targets) != 1 {
		return errors.New("CopyRemoteFile only supports single targets")
	}

	c := pb.NewLocalFileClient(conn)
	// Copy the file to the backup path.
	// Gets root:root as owner with 0644 as perms.
	// Fails if it already exists
	req := &pb.CopyRequest{
		Bucket: "file://" + filepath.Dir(source),
		Key:    filepath.Base(source),
		Destination: &pb.FileWrite{
			Overwrite: false,
			Attrs: &pb.FileAttributes{
				Filename: destination.Filename,
				Attributes: []*pb.FileAttribute{
					{
						Value: &pb.FileAttribute_Mode{
							Mode: uint32(destination.Perms),
						},
					},
					{
						Value: &pb.FileAttribute_Username{
							Username: destination.User,
						},
					},
					{
						Value: &pb.FileAttribute_Group{
							Group: destination.Group,
						},
					},
				},
			},
		},
	}
	_, err := c.Copy(ctx, req)
	if err != nil {
		return fmt.Errorf("copy problem for %s -> %s: %v", source, destination.Filename, err)
	}
	return nil
}
