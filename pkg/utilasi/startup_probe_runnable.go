/*
Copyright 2023 The Kruise Authors.

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

package utilasi

import (
	"context"
	"errors"
	"net/http"
	"os"
	"sync"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// StartupProbeRunnable implement Runnable Start function,
// wait for cache sync, and listen startup probe port
type StartupProbeRunnable struct {
	// mu is used to synchronize Controller setup
	mu sync.Mutex
	// Started is true if the Controller has been Started
	Started          bool
	StartupProbeAddr string
}

func (r *StartupProbeRunnable) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.Started {
		return errors.New("controller was started more than once. This is likely to be caused by being added to a manager multiple times")
	}
	http.HandleFunc("/", func(http.ResponseWriter, *http.Request) {})
	klog.V(3).Infof("StartupProbeRunnable Listening on startup probe port %s", r.StartupProbeAddr)
	r.Started = true
	go func() {
		if err := http.ListenAndServe(r.StartupProbeAddr, nil); err != nil {
			klog.Errorf("Listen and Serve addr(%s) failed: %s", r.StartupProbeAddr, err.Error())
			os.Exit(1)
		}
	}()
	r.mu.Unlock()

	<-ctx.Done()
	klog.V(3).Infof("Shutdown signal received, and stop StartupProbeRunnable")
	return nil
}

var _ manager.LeaderElectionRunnable = &StartupProbeRunnable{}

func (r *StartupProbeRunnable) NeedLeaderElection() bool {
	return false
}
