// Copyright (c) 2018 The Jaeger Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package configmanager

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/jaegertracing/jaeger/internal/metricstest"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/jaegertracing/jaeger/thrift-gen/baggage"
)

type noopManager struct{}

func (noopManager) GetSamplingStrategy(_ context.Context, s string) (*api_v2.SamplingStrategyResponse, error) {
	if s == "failed" {
		return nil, errors.New("failed")
	}
	return &api_v2.SamplingStrategyResponse{StrategyType: api_v2.SamplingStrategyType_PROBABILISTIC}, nil
}

func (noopManager) GetBaggageRestrictions(_ context.Context, s string) ([]*baggage.BaggageRestriction, error) {
	if s == "failed" {
		return nil, errors.New("failed")
	}
	return []*baggage.BaggageRestriction{{BaggageKey: "foo"}}, nil
}

func TestMetrics(t *testing.T) {
	tests := []struct {
		expected []metricstest.ExpectedMetric
		err      error
	}{
		{expected: []metricstest.ExpectedMetric{
			{Name: "collector-proxy", Tags: map[string]string{"result": "ok", "endpoint": "sampling"}, Value: 1},
			{Name: "collector-proxy", Tags: map[string]string{"result": "err", "endpoint": "sampling"}, Value: 0},
			{Name: "collector-proxy", Tags: map[string]string{"result": "ok", "endpoint": "baggage"}, Value: 1},
			{Name: "collector-proxy", Tags: map[string]string{"result": "err", "endpoint": "baggage"}, Value: 0},
		}},
		{expected: []metricstest.ExpectedMetric{
			{Name: "collector-proxy", Tags: map[string]string{"result": "ok", "endpoint": "sampling"}, Value: 0},
			{Name: "collector-proxy", Tags: map[string]string{"result": "err", "endpoint": "sampling"}, Value: 1},
			{Name: "collector-proxy", Tags: map[string]string{"result": "ok", "endpoint": "baggage"}, Value: 0},
			{Name: "collector-proxy", Tags: map[string]string{"result": "err", "endpoint": "baggage"}, Value: 1},
		}, err: errors.New("failed")},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			metricsFactory := metricstest.NewFactory(time.Microsecond)
			defer metricsFactory.Stop()
			mgr := WrapWithMetrics(&noopManager{}, metricsFactory)

			if test.err != nil {
				s, err := mgr.GetSamplingStrategy(context.Background(), test.err.Error())
				require.Nil(t, s)
				assert.EqualError(t, err, test.err.Error())
				b, err := mgr.GetBaggageRestrictions(context.Background(), test.err.Error())
				require.Nil(t, b)
				assert.EqualError(t, err, test.err.Error())
			} else {
				s, err := mgr.GetSamplingStrategy(context.Background(), "")
				require.NoError(t, err)
				require.NotNil(t, s)
				b, err := mgr.GetBaggageRestrictions(context.Background(), "")
				require.NoError(t, err)
				require.NotNil(t, b)
			}
			metricsFactory.AssertCounterMetrics(t, test.expected...)
		})
	}
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
