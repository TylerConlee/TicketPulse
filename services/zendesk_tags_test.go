package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAllTags_SinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/v2/tags")

		username, password, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "test@example.com/token", username)
		assert.Equal(t, "test-token", password)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"tags": []map[string]interface{}{
				{"name": "billing", "count": 42},
				{"name": "urgent", "count": 15},
				{"name": "support", "count": 7},
			},
			"links": map[string]interface{}{
				"next": "",
			},
		})
	}))
	defer server.Close()

	client := createTestZendeskClientWithBaseURL(server)

	tags, err := client.ListAllTags()
	require.NoError(t, err)
	assert.Equal(t, []string{"billing", "urgent", "support"}, tags)
}

func TestListAllTags_MultiplePages(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"tags": []map[string]interface{}{
					{"name": "billing", "count": 42},
					{"name": "urgent", "count": 15},
				},
				"links": map[string]interface{}{
					"next": "https://test.zendesk.com/api/v2/tags?page[size]=100&page[after]=abc",
				},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"tags": []map[string]interface{}{
					{"name": "support", "count": 7},
					{"name": "feature-request", "count": 3},
				},
				"links": map[string]interface{}{
					"next": "",
				},
			})
		}
	}))
	defer server.Close()

	client := createTestZendeskClientWithBaseURL(server)

	tags, err := client.ListAllTags()
	require.NoError(t, err)
	assert.Equal(t, []string{"billing", "urgent", "support", "feature-request"}, tags)
	assert.Equal(t, 2, callCount)
}

func TestListAllTags_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tags":  []map[string]interface{}{},
			"links": map[string]interface{}{"next": ""},
		})
	}))
	defer server.Close()

	client := createTestZendeskClientWithBaseURL(server)

	tags, err := client.ListAllTags()
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestListAllTags_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := createTestZendeskClientWithBaseURL(server)

	tags, err := client.ListAllTags()
	assert.Error(t, err)
	assert.Nil(t, tags)
	assert.Contains(t, err.Error(), "zendesk tags API returned status")
}

func TestListAllTags_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := createTestZendeskClientWithBaseURL(server)

	tags, err := client.ListAllTags()
	assert.Error(t, err)
	assert.Nil(t, tags)
	assert.Contains(t, err.Error(), "401")
}

func TestListAllTags_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{invalid json"))
	}))
	defer server.Close()

	client := createTestZendeskClientWithBaseURL(server)

	tags, err := client.ListAllTags()
	assert.Error(t, err)
	assert.Nil(t, tags)
	assert.Contains(t, err.Error(), "failed to decode tags response")
}

func TestListAllTags_LargeTagSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tags := make([]map[string]interface{}, 100)
		for i := range tags {
			tags[i] = map[string]interface{}{
				"name":  "tag-" + http.StatusText(i%5+200),
				"count": i,
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tags":  tags,
			"links": map[string]interface{}{"next": ""},
		})
	}))
	defer server.Close()

	client := createTestZendeskClientWithBaseURL(server)

	tags, err := client.ListAllTags()
	require.NoError(t, err)
	assert.Len(t, tags, 100)
}
