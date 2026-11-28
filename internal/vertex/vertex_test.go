// Copyright 2026 Google LLC
//
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

package vertex

import (
	"testing"

	"github.com/go-steer/core-agent/v3/pkg/config"
)

func TestProviderFor(t *testing.T) {
	for name, want := range map[string]string{
		"gemini-4-argon":    config.ProviderVertex,
		"gemini-4-neon":     config.ProviderVertex,
		"claude-sonnet-5-5": config.ProviderAnthropicVertex,
		"claude-opus-5-5":   config.ProviderAnthropicVertex,
	} {
		if got := providerFor(name); got != want {
			t.Errorf("providerFor(%q) = %q, want %q", name, got, want)
		}
	}
}
