/*
Copyright 2022 The Kruise Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package entrypoint

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/openkruise/kruise/pkg/util/containerexitpriority"
	"k8s.io/apimachinery/pkg/util/wait"
)

type EntryPointer struct {
	// Command is the original specified command and args.
	command []string
	// wait container exit
	waitContainerExit bool
	// Runner encapsulates running commands.
	runner Runner
	// context
	ctx context.Context
}

func NewEntryPointer(ctx context.Context, cmd []string, waitContainerExit bool) *EntryPointer {
	e := &EntryPointer{
		command:           cmd,
		waitContainerExit: waitContainerExit,
		ctx:               ctx,
		runner:            &realRunner{},
	}
	return e
}

// Runner encapsulates running commands.
type Runner interface {
	Run(ctx context.Context, args ...string) error
}

func (e *EntryPointer) Go() error {
	// 1. wait container exit priority
	if e.waitContainerExit {
		err := e.execWaitContainerExit()
		if err != nil {
			return err
		}
	}

	// 2. execute command
	return e.runner.Run(e.ctx, e.command...)
}

func (e *EntryPointer) execWaitContainerExit() error {
	file := path.Join(containerexitpriority.KruiseDaemonShareRootVolume, containerexitpriority.KruiseContainerExitPriorityFile)
	os.Remove(file)
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	f.Close()

	conditionFunc := func() (done bool, err error) {
		by, err := ioutil.ReadFile(file)
		if err != nil {
			return false, err
		}
		if string(by) != containerexitpriority.KruiseContainerAllowExitFlag {
			return false, nil
		}
		os.Remove(file)
		return true, nil
	}
	return wait.PollImmediateUntil(time.Second, conditionFunc, e.ctx.Done())
}
