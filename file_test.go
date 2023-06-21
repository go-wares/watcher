// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// author: wsfuyibing <websearch@163.com>
// date: 2023-06-21

package watcher

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestNewFile(t *testing.T) {
	var (
		ctx  = context.Background()
		obj  = NewFile()
		call = func(path string, body []byte, err error) {
			t.Logf("path=%s, error=%v, cpu=%d, goroutine=%d", path, err, runtime.NumCPU(), runtime.NumGoroutine())
		}
	)

	obj.SetDuration(time.Second).Add("/tmp/a.txt", call).Add("/tmp/b.txt", call).Start(ctx)
}
