/*
Copyright 2022.

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

package main

import (
	"flag"
	"os"

	"github.com/openkruise/kruise/pkg/util/entrypoint"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

var (
	waitContainerExit = flag.Bool("wait-container-exit", false, "whether exec wait container exit priority")
	ep                = flag.String("entrypoint", "", "Command is the original specified command and args")
)

func main() {
	flag.Parse()
	var cmd []string
	if *ep != "" {
		cmd = []string{*ep}
		cmd = append(cmd, flag.Args()...)
	}
	ctx := signals.SetupSignalHandler()
	entrypointer := entrypoint.NewEntryPointer(ctx, cmd, *waitContainerExit)
	err := entrypointer.Go()
	if err != nil {
		os.Exit(1)
	}
}
