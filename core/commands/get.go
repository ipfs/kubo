package commands

import (
	"archive/tar"
	"bytes"
	"io"
	p "path"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	dag "github.com/jbenet/go-ipfs/merkledag"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	upb "github.com/jbenet/go-ipfs/unixfs/pb"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
)

var GetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Download IPFS objects",
		ShortDescription: `
Retrieves the object named by <ipfs-path> and stores the data to disk.

By default, the output will be stored at ./<ipfs-path>, but an alternate path
can be specified with '--output=<path>' or '-o=<path>'.

To output a TAR archive instead of unpacked files, use '--archive' or '-a'.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to be outputted").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("output", "o", "The path where output should be stored"),
		cmds.BoolOption("archive", "a", "Output a TAR archive"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		reader, err := get(node, req.Arguments())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(reader)
	},

	// TODO: create a PostRun that splits the archive up into files
}

func get(node *core.IpfsNode, paths []string) (io.Reader, error) {
	reader := &getReader{signalChan: make(chan struct{})}
	writer := tar.NewWriter(&reader.buf)

	go func() {
		for _, path := range paths {
			_, err := copyFile(node, writer, path, nil, reader.signalChan)
			if err != nil {
				log.Error(err)
				return
			}
		}

		err := writer.Flush()
		if err != nil {
			log.Error(err)
			return
		}

		reader.Close()
		reader.Signal()
	}()

	return reader, nil
}

func copyFile(node *core.IpfsNode, writer *tar.Writer, path string, dagnode *dag.Node, signal chan struct{}) (int64, error) {
	var err error
	if dagnode == nil {
		dagnode, err = node.Resolver.ResolvePath(path)
		if err != nil {
			return 0, err
		}
	}

	pb := new(upb.Data)
	err = proto.Unmarshal(dagnode.Data, pb)
	if err != nil {
		return 0, err
	}

	written := int64(0)
	if pb.GetType() == upb.Data_Directory {
		err = writer.WriteHeader(&tar.Header{
			Name:     path,
			Typeflag: tar.TypeDir,
			Mode:     0777,
			// TODO: set mode, dates, etc. when added to unixFS
		})
		if err != nil {
			return 0, err
		}

		for _, link := range dagnode.Links {
			n, err := copyFile(node, writer, p.Join(path, link.Name), link.Node, signal)
			if err != nil {
				return 0, err
			}
			written += n
		}
		return written, nil

	} else {
		err = writer.WriteHeader(&tar.Header{
			Name:     path,
			Size:     int64(pb.GetFilesize()),
			Typeflag: tar.TypeReg,
			Mode:     0644,
			// TODO: set mode, dates, etc. when added to unixFS
		})
		if err != nil {
			return 0, err
		}

		reader, err := uio.NewDagReader(dagnode, node.DAG)
		if err != nil {
			return 0, err
		}

		buf := make([]byte, 32*1024)
		for {
			nr, err := reader.Read(buf)
			if nr > 0 {
				nw, err := writer.Write(buf[:nr])
				if err != nil {
					return written, err
				}
				written += int64(nw)
				signal <- struct{}{}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return written, err
			}
		}
		return written, nil
	}
}

type getReader struct {
	buf        bytes.Buffer
	closed     bool
	signalChan chan struct{}
}

func (i *getReader) Read(p []byte) (int, error) {
	<-i.signalChan
	n, err := i.buf.Read(p)
	if err == io.EOF && !i.closed {
		return n, nil
	}
	return n, err
}

func (i *getReader) Signal() {
	i.signalChan <- struct{}{}
}

func (i *getReader) Close() {
	i.closed = true
}
