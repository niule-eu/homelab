package effects

import (
	"errors"
	"fmt"
	"os"
)

type Effect interface {
	Apply() error
}

type FileWrite struct {
	Path        string
	Content     []byte
	Permissions os.FileMode
}

func (fw FileWrite) Apply() error {
	return os.WriteFile(fw.Path, fw.Content, fw.Permissions)
}

type FileDelete struct {
	Path string
}

func (fd FileDelete) Apply() error {
	return os.Remove(fd.Path)
}

type Stdout struct {
	Message string
}

func (s Stdout) Apply() error {
	fmt.Println(s.Message)
	return nil
}

type NoOp struct{}

func (n NoOp) Apply() error {
	return nil
}

type Compound struct {
	Effects  []Effect
	FailFast bool
}

func (c Compound) Apply() error {
	var errs []error
	for _, e := range c.Effects {
		if err := e.Apply(); err != nil {
			if c.FailFast {
				return err
			}
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func Invoke(failFast bool, effects ...Effect) error {
	return Compound{Effects: effects, FailFast: failFast}.Apply()
}
