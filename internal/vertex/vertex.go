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

// Package vertex opens Claude and Gemini models on Vertex AI through
// core-agent's model registry, with the pricing catalog that goes with
// them. The provider is chosen by model name: gemini-* goes to Vertex AI
// Gemini, anything else to Claude on Vertex AI.
package vertex

import (
	"context"
	"fmt"
	"strings"
	"sync"

	adkmodel "google.golang.org/adk/v2/model"

	"github.com/go-steer/core-agent/v3/pkg/config"
	"github.com/go-steer/core-agent/v3/pkg/models"
	_ "github.com/go-steer/core-agent/v3/pkg/models/anthropic" // registers anthropic-vertex
	_ "github.com/go-steer/core-agent/v3/pkg/models/gemini"    // registers vertex (Gemini)
	"github.com/go-steer/core-agent/v3/pkg/pricing"
	"github.com/go-steer/core-agent/v3/pkg/usage"
)

// Models opens models in one Vertex AI project and region.
type Models struct {
	project, region string
	// Pricing prices usage on every provider; Backends maps each opened
	// model's name to its billing backend.
	Pricing *pricing.Catalog

	mu        sync.Mutex
	providers map[string]models.Provider // config provider -> provider
	backends  map[string]string          // model name -> backend
}

// Open prepares project and region; providers are resolved on first use,
// with Application Default Credentials.
func Open(project, region string) (*Models, error) {
	catalog, err := usage.NewCatalogFromConfig(config.DefaultConfig(), "", "")
	if err != nil {
		return nil, fmt.Errorf("vertex: pricing: %w", err)
	}
	return &Models{project: project, region: region, Pricing: catalog,
		providers: map[string]models.Provider{}, backends: map[string]string{}}, nil
}

// providerFor returns the core-agent provider name for a model.
func providerFor(name string) string {
	if strings.HasPrefix(name, "gemini-") {
		return config.ProviderVertex
	}
	return config.ProviderAnthropicVertex
}

// Model opens one model by name, e.g. claude-sonnet-5-5 or gemini-4-argon.
func (m *Models) Model(ctx context.Context, name string) (adkmodel.LLM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	kind := providerFor(name)
	p, ok := m.providers[kind]
	if !ok {
		cfg := config.DefaultConfig()
		cfg.Model.Provider = kind
		cfg.Model.Name = name
		if kind == config.ProviderVertex {
			cfg.Model.Vertex = &config.VertexConfig{Project: m.project, Location: m.region}
		} else {
			cfg.Model.Anthropic = &config.AnthropicConfig{Vertex: &config.VertexConfig{Project: m.project, Location: m.region}}
		}
		var err error
		if p, err = models.Resolve(cfg); err != nil {
			return nil, fmt.Errorf("vertex: %s: %w", name, err)
		}
		m.providers[kind] = p
	}
	llm, err := p.Model(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("vertex: %s: %w", name, err)
	}
	m.backends[name] = models.BackendOf(p)
	return llm, nil
}

// Backends maps every model opened so far to its billing backend, for
// review.Options.Backends.
func (m *Models) Backends() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]string, len(m.backends))
	for k, v := range m.backends {
		out[k] = v
	}
	return out
}

// Backend is the billing backend of an opened model, or "".
func (m *Models) Backend(name string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.backends[name]
}
