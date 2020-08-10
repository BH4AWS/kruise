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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func touch(filename string) error {
	f, err := os.Create(filename)
	if err == nil {
		f.Close()
	}
	return err
}

func TestCleanLogFiles(t *testing.T) {
	dir, _ := ioutil.TempDir("/tmp", "exporter")

	for i := 0; i < 10; i++ {
		touch(fmt.Sprintf("%v/log.INFO.%v", dir, i))
	}

	pattern := fmt.Sprintf("%v/log.INFO.*", dir)
	files, err := filepath.Glob(pattern)
	if err != nil || len(files) != 10 {
		t.Fatalf("init files failed")
	}

	err = cleanFiles(5, pattern)
	files, err = filepath.Glob(pattern)
	if err != nil || len(files) != 5 {
		t.Fatalf("clean files failed, expect %v actual %v", 5, len(files))
	}

	err = cleanFiles(1, pattern)
	files, err = filepath.Glob(pattern)
	filename := fmt.Sprintf("%v/log.INFO.%v", dir, 9)
	if len(files) != 1 || files[0] != filename {
		t.Fatalf("clean files failed, expect [%v] actual %v", filename, files)
	}
}
