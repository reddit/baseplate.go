package metricsbp

import (
	"bytes"
	"io"

	"github.com/reddit/baseplate.go/log"
)

type bufferedWriter struct {
	buf  bytes.Buffer
	w    io.Writer
	size int
}

func newBufferedWriter(w io.Writer, size int) *bufferedWriter {
	bufWriter := &bufferedWriter{
		w:    w,
		size: size,
	}
	if size > 0 {
		bufWriter.buf.Grow(size)
	}
	return bufWriter
}

func (bw *bufferedWriter) Flush() error {
	if bw.buf.Len() == 0 {
		return nil
	}

	_, err := bw.w.Write(bw.buf.Bytes())
	bw.buf.Reset()
	return err
}

func (bw *bufferedWriter) Write(p []byte) (n int, err error) {
	if bw.buf.Len()+len(p) > bw.size {
		err = bw.Flush()
		if err != nil {
			return
		}
	}
	return bw.buf.Write(p)
}

func (bw *bufferedWriter) doWrite(src io.WriterTo, logger log.KitWrapper) (err error) {
	defer func() {
		if err != nil {
			logger.Log("during", "WriteTo", "err", err)
		}
	}()

	if _, err := src.WriteTo(bw); err != nil {
		return err
	}
	return bw.Flush()
}
