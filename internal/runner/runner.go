package runner

import (
	"bytes"
	"fmt"
	"os/exec"
)

type Result struct {
	Stdout string
	Stderr string
	Code   int
}

func Run(dir string, argv ...string) (*Result, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &outBuf, &errBuf
	err := cmd.Run()
	exit := 0
	if ee, ok := err.(*exec.ExitError); ok {
		exit = ee.ExitCode()
		err = nil // we still want Result
	}
	return &Result{Stdout: outBuf.String(), Stderr: errBuf.String(), Code: exit}, err
}

func Fatalf(dir string, argv ...string) error {
	if _, err := exec.LookPath(argv[0]); err != nil {
		return fmt.Errorf("executable %q not in PATH", argv[0])
	}
	_, err := Run(dir, argv...)
	return err
}
