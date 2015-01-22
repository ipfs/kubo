package commands

import (
	"archive/tar"
	"bytes"
	"io"
	p "path"
	"sync"

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

		reader, err := get(node, req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(reader)
	},
}

func get(node *core.IpfsNode, path string) (io.Reader, error) {
	buf := NewBufReadWriter()

	go func() {
		err := copyFilesAsTar(node, buf, path)
		if err != nil {
			log.Error(err)
			return
		}
	}()

	return buf, nil
}

func copyFilesAsTar(node *core.IpfsNode, buf *bufReadWriter, path string) error {
	writer := tar.NewWriter(buf)

	err := _copyFilesAsTar(node, writer, buf, path, nil)
	if err != nil {
		return err
	}

	err = writer.Flush()
	if err != nil {
		return err
	}
	buf.Close()
	buf.Signal()
	return nil
}

func _copyFilesAsTar(node *core.IpfsNode, writer *tar.Writer, buf *bufReadWriter, path string, dagnode *dag.Node) error {
	var err error
	if dagnode == nil {
		dagnode, err = node.Resolver.ResolvePath(path)
		if err != nil {
			return err
		}
	}

	pb := new(upb.Data)
	err = proto.Unmarshal(dagnode.Data, pb)
	if err != nil {
		return err
	}

	if pb.GetType() == upb.Data_Directory {
		buf.mutex.Lock()
		err = writer.WriteHeader(&tar.Header{
			Name:     path,
			Typeflag: tar.TypeDir,
			Mode:     0777,
			// TODO: set mode, dates, etc. when added to unixFS
		})
		buf.mutex.Unlock()
		if err != nil {
			return err
		}

		for _, link := range dagnode.Links {
			err := _copyFilesAsTar(node, writer, buf, p.Join(path, link.Name), link.Node)
			if err != nil {
				return err
			}
		}

		return nil
	}

	buf.mutex.Lock()
	err = writer.WriteHeader(&tar.Header{
		Name:     path,
		Size:     int64(pb.GetFilesize()),
		Typeflag: tar.TypeReg,
		Mode:     0644,
		// TODO: set mode, dates, etc. when added to unixFS
	})
	buf.mutex.Unlock()
	if err != nil {
		return err
	}

	reader, err := uio.NewDagReader(dagnode, node.DAG)
	if err != nil {
		return err
	}

	_, err = syncCopy(writer, reader, buf)
	if err != nil {
		return err
	}

	return nil
}

type bufReadWriter struct {
	buf        bytes.Buffer
	closed     bool
	signalChan chan struct{}
	mutex      *sync.Mutex
}

func NewBufReadWriter() *bufReadWriter {
	return &bufReadWriter{
		signalChan: make(chan struct{}),
		mutex:      &sync.Mutex{},
	}
}

func (i *bufReadWriter) Read(p []byte) (int, error) {
	<-i.signalChan
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if i.buf.Len() == 0 {
		if i.closed {
			return 0, io.EOF
		}
		return 0, nil
	}

	n, err := i.buf.Read(p)
	if err == io.EOF && !i.closed || i.buf.Len() > 0 {
		return n, nil
	}
	return n, err
}

func (i *bufReadWriter) Write(p []byte) (int, error) {
	return i.buf.Write(p)
}

func (i *bufReadWriter) Signal() {
	i.signalChan <- struct{}{}
}

func (i *bufReadWriter) Close() error {
	i.closed = true
	return nil
}

func syncCopy(writer io.Writer, reader io.Reader, buf *bufReadWriter) (int64, error) {
	written := int64(0)
	copyBuf := make([]byte, 32*1024)
	for {
		nr, err := reader.Read(copyBuf)
		if nr > 0 {
			buf.mutex.Lock()
			nw, err := writer.Write(copyBuf[:nr])
			buf.mutex.Unlock()
			if err != nil {
				return written, err
			}
			written += int64(nw)
			buf.Signal()
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
