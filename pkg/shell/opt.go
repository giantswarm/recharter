package shell

import (
	"bytes"
	"os/exec"
)

type option interface {
	PreRun(*exec.Cmd)
	PostRun(*exec.Cmd)
}

type captureStdoutToOption struct {
	bs  *[]byte
	buf *bytes.Buffer
}

func (o *captureStdoutToOption) PreRun(cmd *exec.Cmd) {
	o.buf = new(bytes.Buffer)
	cmd.Stdout = o.buf
}

func (o *captureStdoutToOption) PostRun(cmd *exec.Cmd) {
	*o.bs = o.buf.Bytes()
}

type dirOption struct {
	dir string
}

func (o *dirOption) PreRun(cmd *exec.Cmd) {
	cmd.Dir = o.dir
}

func (o *dirOption) PostRun(cmd *exec.Cmd) {
}
