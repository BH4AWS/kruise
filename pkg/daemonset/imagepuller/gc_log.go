/*
Copyright 2019 The Kruise Authors.

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

package imagepuller

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/klog"
)

func cleanFiles(keepFiles int, pattern string) error {
	files, err := filepath.Glob(pattern)
	if err != nil {
		klog.Warningf("failed to list files %v", pattern)
		return err
	}
	for i := 0; i < len(files)-keepFiles; i++ {
		klog.Infof("remove file %v", files[i])
		os.Remove(files[i])
	}
	return nil
}

// CleanKlogFiles remove old logs
func CleanKlogFiles(keepFiles int) {
	logDir := flag.Lookup("log_dir").Value.String()
	cleanFiles(keepFiles, fmt.Sprintf("%s/*log.INFO*", logDir))
	cleanFiles(keepFiles, fmt.Sprintf("%s/*log.WARNING*", logDir))
	cleanFiles(keepFiles, fmt.Sprintf("%s/*log.ERROR*", logDir))
	cleanFiles(keepFiles, fmt.Sprintf("%s/*log.FATAL*", logDir))
}
