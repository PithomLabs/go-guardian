package token

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/shaj13/go-guardian/auth"
	"github.com/shaj13/go-guardian/store"
)

func TestNewCahced(t *testing.T) {
	table := []struct {
		name        string
		panic       bool
		expectedErr bool
		cache       store.Cache
		authFunc    AuthenticateFunc
		info        interface{}
		token       string
	}{
		{
			name:        "it return error when cache load return error",
			expectedErr: true,
			panic:       false,
			cache:       make(mockCache),
			token:       "error",
			authFunc:    NoOpAuthenticate,
			info:        nil,
		},
		{
			name:        "it return error when user authenticate func return error",
			expectedErr: true,
			cache:       make(mockCache),
			authFunc:    NoOpAuthenticate,
			panic:       false,
			info:        nil,
		},
		{
			name:        "it return error when cache store return error",
			expectedErr: true,
			cache:       make(mockCache),
			authFunc:    func(_ context.Context, _ *http.Request, _ string) (auth.Info, error) { return nil, nil },
			token:       "store-error",
			panic:       false,
			info:        nil,
		},
		{
			name:        "it return error when cache return invalid type",
			expectedErr: true,
			cache:       make(mockCache),
			authFunc:    func(_ context.Context, _ *http.Request, _ string) (auth.Info, error) { return nil, nil },
			panic:       false,
			info:        "sample-data",
			token:       "valid",
		},
		{
			name:        "it return user when token cached",
			expectedErr: false,
			cache:       make(mockCache),
			authFunc:    NoOpAuthenticate,
			panic:       false,
			info:        auth.NewDefaultUser("1", "1", nil, nil),
			token:       "valid-user",
		},
		{
			name:        "it panic when Authenticate func nil",
			expectedErr: false,
			panic:       true,
			info:        nil,
		},
		{
			name:        "it panic when Cache nil",
			expectedErr: false,
			authFunc:    NoOpAuthenticate,
			panic:       true,
			info:        nil,
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			if tt.panic {
				assert.Panics(t, func() {
					New(tt.authFunc, tt.cache)
				})
				return
			}

			strategy := New(tt.authFunc, tt.cache)
			r, _ := http.NewRequest("GET", "/", nil)
			r.Header.Set("Authorization", "Bearer "+tt.token)
			_ = tt.cache.Store(tt.token, tt.info, r)
			info, err := strategy.Authenticate(r.Context(), r)
			if tt.expectedErr {
				assert.Error(t, err)
				return
			}
			assert.Equal(t, tt.info, info)
		})
	}
}

func TestCahcedTokenAppend(t *testing.T) {
	cache := make(mockCache)
	strategy := &cachedToken{cache: cache}
	info := auth.NewDefaultUser("1", "2", nil, nil)
	strategy.Append("test-append", info, nil)
	cachedInfo, ok, _ := cache.Load("test-append", nil)
	assert.True(t, ok)
	assert.Equal(t, info, cachedInfo)
}

func TestCahcedTokenChallenge(t *testing.T) {
	strategy := &cachedToken{
		typ: Bearer,
	}

	got := strategy.Challenge("Test Realm")
	expected := `Bearer realm="Test Realm", title="Bearer Token Based Authentication Scheme"`

	assert.Equal(t, expected, got)
}

func BenchmarkCachedToken(b *testing.B) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer token")

	cache := make(mockCache)
	cache.Store("token", auth.NewDefaultUser("benchmark", "1", nil, nil), r)

	strategy := New(NoOpAuthenticate, cache)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := strategy.Authenticate(r.Context(), r)
			if err != nil {
				b.Error(err)
			}
		}
	})
}

type mockCache map[string]interface{}

func (m mockCache) Load(key string, _ *http.Request) (interface{}, bool, error) {
	if key == "error" {
		return nil, false, fmt.Errorf("Load Error")
	}
	v, ok := m[key]
	return v, ok, nil
}

func (m mockCache) Store(key string, value interface{}, _ *http.Request) error {
	if key == "store-error" {
		return fmt.Errorf("Store Error")
	}
	m[key] = value
	return nil
}
func (m mockCache) Keys() []string { return nil }

func (m mockCache) Delete(key string, _ *http.Request) error {
	return nil
}
