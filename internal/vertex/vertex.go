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

// Package vertex opens Claude models on Vertex AI through core-agent's
// model registry, with the pricing catalog that goes with them.
package vertex

import (
	"context"
	"fmt"

	adkmodel "google.golang.org/adk/v2/model"

	"github.com/go-steer/core-agent/v3/pkg/config"
	"github.com/go-steer/core-agent/v3/pkg/models"
	_ "github.com/go-steer/core-agent/v3/pkg/models/anthropic" // registers anthropic-vertex
	"github.com/go-steer/core-agent/v3/pkg/pricing"
	"github.com/go-steer/core-agent/v3/pkg/usage"
)

// Claude is a Vertex AI project and region serving Claude models.
type Claude struct {
	provider models.Provider
	// Pricing and Backend price usage on this provider.
	Pricing *pricing.Catalog
	Backend string
}

// Open resolves the anthropic-vertex provider for project and region,
// using Application Default Credentials.
func Open(project, region string) (*Claude, error) {
	cfg := config.DefaultConfig()
	cfg.Model.Provider = config.ProviderAnthropicVertex
	cfg.Model.Name = "claude-sonnet-5-5" // any valid name; models are opened by name below
	cfg.Model.Anthropic = &config.AnthropicConfig{Vertex: &config.VertexConfig{Project: project, Location: region}}
	provider, err := models.Resolve(cfg)
	if err != nil {
		return nil, fmt.Errorf("vertex: %w", err)
	}
	catalog, err := usage.NewCatalogFromConfig(cfg, "", "")
	if err != nil {
		return nil, fmt.Errorf("vertex: pricing: %w", err)
	}
	return &Claude{provider: provider, Pricing: catalog, Backend: models.BackendOf(provider)}, nil
}

// Model opens one model by name, e.g. claude-sonnet-5-5.
func (c *Claude) Model(ctx context.Context, name string) (adkmodel.LLM, error) {
	m, err := c.provider.Model(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("vertex: %s: %w", name, err)
	}
	return m, nil
}
