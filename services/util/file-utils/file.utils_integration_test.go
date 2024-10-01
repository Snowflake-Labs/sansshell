/*
Copyright (c) 2024 Snowflake Inc. All rights reserved.

	Licensed under the Apache License, Version 2.0 (the
	"License"); you may not use this file except in compliance
	with the License.  You may obtain a copy of the License at

	  http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing,
	software distributed under the License is distributed on an
	"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
	KIND, either express or implied.  See the License for the
	specific language governing permissions and limitations
	under the License.
*/
package file_utils

import (
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

func TestIntegration_OpenForOverwrite(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test")
	}

	t.Run("It should open file and being able read it", func(t *testing.T) {
		// ARRANGE
		filePath := "./__testdata__/TestIntegrationOpenForOverwrite.test"
		expectedFileContent := "Hello world"

		// ACT
		f, err := OpenForOverwrite(filePath)
		if err != nil {
			t.Errorf("Unexpected open error: %s", err.Error())
			return
		}
		defer f.Close()

		reader := io.Reader(f)
		result, err := io.ReadAll(reader)

		// ASSERT
		if err != nil {
			t.Errorf("Unexpected read error: %s", err.Error())
			return
		}

		text := string(result)
		if text != expectedFileContent {
			t.Errorf("Expected \"%s\", but got \"%s\"", expectedFileContent, text)
			return
		}
	})

	t.Run("It should overwrite file, but keep the same mode", func(t *testing.T) {
		// ARRANGE
		sourceFilePath := "./__testdata__/TestIntegrationOpenForOverwrite.test"
		newFileContent := "new content"
		expectedFileMode := os.FileMode(0767)

		// make source file tmp copy
		sourceFile, err := os.Open(sourceFilePath)
		if err != nil {
			t.Errorf("Unexpected open source file error: %s", err.Error())
			return
		}
		tmpPath := "./__testdata__/tmp_TestIntegrationOpenForOverwrite.test"
		destinationFile, err := os.Create(tmpPath)
		if err != nil {
			t.Errorf("Unexpected create tmp file error: %s", err.Error())
			return
		}
		_, err = io.Copy(destinationFile, sourceFile)
		if err != nil {
			t.Errorf("Unexpected copy file error: %s", err.Error())
		}

		err = destinationFile.Sync()
		if err != nil {
			t.Errorf("Unexpected sync file error: %s", err.Error())
			return
		}

		err = sourceFile.Close()
		if err != nil {
			t.Errorf("Unexpected close source file error: %s", err.Error())
			return
		}
		err = destinationFile.Close()
		if err != nil {
			t.Errorf("Unexpected close destination file error: %s", err.Error())
			return
		}
		defer (func() {
			err := os.Remove(tmpPath)
			if err != nil {
				t.Errorf("Unexpected remove tmp file error: %s", err.Error())
				return
			}
		})()
		// set for file expected file mode
		err = os.Chmod(tmpPath, expectedFileMode)
		if err != nil {
			t.Errorf("Unexpected chmod error: %s", err.Error())
			return
		}

		// ACT
		f, err := OpenForOverwrite(tmpPath)
		if err != nil {
			t.Errorf("Unexpected open error: %s", err.Error())
			return
		}

		// write new content
		_, err = f.Write([]byte(newFileContent))
		if err != nil {
			t.Errorf("Unexpected write error: %s", err.Error())
			return
		}

		err = f.Close()
		if err != nil {
			t.Errorf("Unexpected close error: %s", err.Error())
			return
		}

		// ASSERT
		// get file mode
		fileInfo, err := os.Stat(tmpPath)
		if err != nil {
			t.Errorf("Unexpected stat error: %s", err.Error())
			return
		}

		if fileInfo.Mode() != expectedFileMode {
			t.Errorf("Expected %s, but got %s", expectedFileMode, fileInfo.Mode())
			return
		}

		// read file content
		f, err = os.Open(tmpPath)
		if err != nil {
			t.Errorf("Unexpected open error: %s", err.Error())
			return
		}
		defer f.Close()

		content := make([]byte, len(newFileContent))
		_, err = f.Read(content)
		if err != nil {
			t.Errorf("Unexpected read error: %s", err.Error())
			return
		}

		if string(content) != newFileContent {
			t.Errorf("Expected %s, but got %s", newFileContent, content)
			return
		}
	})
}

func TestIntegration_ExclusiveLockFile(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test")
	}

	t.Run("It should lock file second time only, when first lock was released", func(t *testing.T) {
		// ARRANGE
		filePath := "./__testdata__/TestIntegrationOpenForOverwrite.test"

		// preparations
		f, err := os.Open(filePath)
		if err != nil {
			t.Errorf("Unexpected open source file error: %s", err.Error())
			return
		}
		defer f.Close()

		// ACT
		var wg sync.WaitGroup
		var firstUnlockTime time.Time
		var secondLockTime time.Time

		wg.Add(2)
		go (func() {
			releaseFn, err := ExclusiveLockFile(f)
			if err != nil {
				t.Errorf("Unexpected lock error: %s", err.Error())
				return
			}
			time.Sleep(5 * time.Second) // Simulate work

			firstUnlockTime = time.Now()
			_ = releaseFn()
			wg.Done()
		})()
		go (func() {
			releaseFn, err := ExclusiveLockFile(f)
			secondLockTime = time.Now()
			if err != nil {
				t.Errorf("Unexpected lock error: %s", err.Error())
				return
			}
			_ = releaseFn()
			wg.Done()
		})()
		wg.Wait()

		// ASSERT
		if secondLockTime.After(firstUnlockTime) {
			t.Errorf("Second lock should happen after first unlock, but got %s and %s", firstUnlockTime, secondLockTime)
			return
		}
	})
}
