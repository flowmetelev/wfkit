package webflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetCMSSnapshotParsesDatabaseCollections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/sites/wfkit/dom" {
			t.Fatalf("unexpected path %s", got)
		}
		if got := r.URL.Query().Get("workflow"); got != "cms" {
			t.Fatalf("expected workflow=cms, got %q", got)
		}
		if got := r.Header.Get("x-xsrf-token"); got != "csrf-token" {
			t.Fatalf("expected csrf token header, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"siteName":       "wfkit",
			"maxCollections": 40,
			"maxLocales":     1,
			"database": map[string]interface{}{
				"_id":          "db-1",
				"name":         "WFKit Database",
				"maxItemLimit": 10000,
				"collections": []map[string]interface{}{
					{
						"_id":          "coll-1",
						"name":         "Articles",
						"slug":         "articles",
						"singularName": "Article",
						"database":     "db-1",
						"detailPage":   "page-1",
						"fields": []map[string]interface{}{
							{"id": "field-1", "name": "Name", "slug": "name", "type": "PlainText", "required": true},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	snapshot, err := getCMSSnapshotFromURL(context.Background(), server.URL+"/api/sites/wfkit/dom?workflow=cms&t=1", "csrf-token", "cookie=value")
	if err != nil {
		t.Fatalf("GetCMSSnapshot returned error: %v", err)
	}
	if snapshot.Database == nil || snapshot.Database.ID != "db-1" {
		t.Fatalf("expected database db-1, got %#v", snapshot.Database)
	}
	if len(snapshot.Database.Collections) != 1 {
		t.Fatalf("expected one collection, got %d", len(snapshot.Database.Collections))
	}
	if snapshot.Database.Collections[0].Slug != "articles" {
		t.Fatalf("expected articles slug, got %q", snapshot.Database.Collections[0].Slug)
	}
}

func TestGetCMSCollectionItemsParsesCollectionPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/collections/coll-1/items" {
			t.Fatalf("unexpected path %s", got)
		}
		if got := r.URL.Query().Get("target"); got != "staging" {
			t.Fatalf("expected target=staging, got %q", got)
		}
		if got := r.URL.Query().Get("format"); got != "withoutHeavyFields" {
			t.Fatalf("expected format query, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "items",
			"items": []map[string]interface{}{
				{
					"_id": "item-1",
					"fieldData": map[string]interface{}{
						"name": "Hello",
						"slug": "hello",
					},
				},
			},
			"total": 1,
		})
	}))
	defer server.Close()

	payload, err := getCMSCollectionItemsFromURL(context.Background(), server.URL+"/api/v1/collections/coll-1/items?target=staging&format=withoutHeavyFields", "csrf-token", "cookie=value")
	if err != nil {
		t.Fatalf("getCMSCollectionItemsFromURL returned error: %v", err)
	}
	if payload.Total != 1 || len(payload.Items) != 1 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if got := firstNestedString(payload.Items[0], "fieldData", "slug"); got != "hello" {
		t.Fatalf("expected hello slug, got %q", got)
	}
}

func TestListCMSCollectionsHandlesMissingDatabase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "workflow=cms") {
			t.Fatalf("expected workflow query, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"siteName": "wfkit",
			"database": nil,
		})
	}))
	defer server.Close()

	snapshot, err := getCMSSnapshotFromURL(context.Background(), server.URL+"/api/sites/wfkit/dom?workflow=cms&t=1", "csrf-token", "cookie=value")
	if err != nil {
		t.Fatalf("getCMSSnapshotFromURL returned error: %v", err)
	}
	var collections []CMSCollection
	if snapshot.Database != nil {
		collections = snapshot.Database.Collections
	}
	if len(collections) != 0 {
		t.Fatalf("expected no collections, got %d", len(collections))
	}
}

func TestCreateCMSItemSendsFieldsPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/collections/coll-1/items" {
			t.Fatalf("unexpected path %s", got)
		}
		var body struct {
			Fields map[string]interface{} `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Fields["name"] != "Hello" {
			t.Fatalf("expected Hello field, got %#v", body.Fields)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"_id": "item-1", "name": "Hello"},
			},
		})
	}))
	defer server.Close()

	item, err := createCMSItemFromURL(context.Background(), server.URL+"/api/v1/collections/coll-1/items", "csrf-token", "cookie=value", map[string]interface{}{"name": "Hello"})
	if err != nil {
		t.Fatalf("createCMSItemFromURL returned error: %v", err)
	}
	if item["_id"] != "item-1" {
		t.Fatalf("unexpected created item: %#v", item)
	}
}

func TestUpdateCMSItemSendsFieldsPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Method; got != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", got)
		}
		if got := r.URL.Path; got != "/api/v1/collections/coll-1/items/item-1" {
			t.Fatalf("unexpected path %s", got)
		}
		var body struct {
			Fields map[string]interface{} `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Fields["name"] != "Updated" {
			t.Fatalf("expected Updated field, got %#v", body.Fields)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"updated_item": map[string]interface{}{
				"_id":  "item-1",
				"name": "Updated",
			},
		})
	}))
	defer server.Close()

	item, err := updateCMSItemFromURL(context.Background(), server.URL+"/api/v1/collections/coll-1/items/item-1", "csrf-token", "cookie=value", map[string]interface{}{"name": "Updated"})
	if err != nil {
		t.Fatalf("updateCMSItemFromURL returned error: %v", err)
	}
	if item["name"] != "Updated" {
		t.Fatalf("unexpected updated item: %#v", item)
	}
}

func TestDeleteCMSItemUsesDeleteMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Method; got != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", got)
		}
		if got := r.URL.Path; got != "/api/v1/collections/coll-1/items/item-1" {
			t.Fatalf("unexpected path %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deleted":1}`))
	}))
	defer server.Close()

	if err := deleteCMSItemByURL(context.Background(), server.URL+"/api/v1/collections/coll-1/items/item-1", "csrf-token", "cookie=value", "item-1"); err != nil {
		t.Fatalf("deleteCMSItemByURL returned error: %v", err)
	}
}
