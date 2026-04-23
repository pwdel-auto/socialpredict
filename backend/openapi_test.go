package main

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"socialpredict/handlers"
)

func TestOpenAPISpecValidates(t *testing.T) {
	doc := loadOpenAPIDoc(t)
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI document: %v", err)
	}
}

func TestEmbeddedOpenAPIAndSwaggerAssetsRemainValid(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("docs", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read checked-in OpenAPI spec: %v", err)
	}
	if !bytes.Equal(openAPISpec, want) {
		t.Fatal("embedded OpenAPI spec does not match docs/openapi.yaml")
	}

	for _, assetPath := range []string{"swagger-ui/index.html", "swagger-ui/swagger-initializer.js"} {
		asset, err := fs.ReadFile(swaggerUIFS, assetPath)
		if err != nil {
			t.Fatalf("read embedded asset %s: %v", assetPath, err)
		}
		if len(asset) == 0 {
			t.Fatalf("embedded asset %s is empty", assetPath)
		}
	}
}

func TestOpenAPIOperationsHaveOperationIDs(t *testing.T) {
	doc := loadOpenAPIDoc(t)

	var missing []string
	for path, pathItem := range doc.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			if operation.OperationID == "" {
				missing = append(missing, strings.ToUpper(method)+" "+path)
			}
		}
	}
	if len(missing) == 0 {
		return
	}

	sort.Strings(missing)
	t.Fatalf("OpenAPI operations missing operationId:\n%s", strings.Join(missing, "\n"))
}

func TestOpenAPIReasonResponseMatchesSharedVocabulary(t *testing.T) {
	doc := loadOpenAPIDoc(t)

	reasonSchemaRef := doc.Components.Schemas["FailureReason"]
	if reasonSchemaRef == nil || reasonSchemaRef.Value == nil {
		t.Fatal("components.schemas.FailureReason is missing")
	}

	reasonResponseRef := doc.Components.Schemas["ReasonResponse"]
	if reasonResponseRef == nil || reasonResponseRef.Value == nil {
		t.Fatal("components.schemas.ReasonResponse is missing")
	}

	reasonPropertyRef := reasonResponseRef.Value.Properties["reason"]
	if reasonPropertyRef == nil {
		t.Fatal("components.schemas.ReasonResponse.properties.reason is missing")
	}
	if reasonPropertyRef.Ref != "#/components/schemas/FailureReason" {
		t.Fatalf("ReasonResponse.reason should reference FailureReason, got %q", reasonPropertyRef.Ref)
	}

	got := enumStrings(reasonSchemaRef.Value.Enum)
	want := make([]string, 0, len(handlers.CanonicalFailureReasons()))
	for _, reason := range handlers.CanonicalFailureReasons() {
		want = append(want, string(reason))
	}
	sort.Strings(got)
	sort.Strings(want)
	if !equalStringSlices(got, want) {
		t.Fatalf("FailureReason enum mismatch:\nwant: %v\ngot:  %v", want, got)
	}

	allowed := make(map[string]struct{}, len(got))
	for _, reason := range got {
		allowed[reason] = struct{}{}
	}

	if example, ok := reasonSchemaRef.Value.Example.(string); ok {
		if _, found := allowed[example]; !found {
			t.Fatalf("FailureReason example %q is not in the shared vocabulary", example)
		}
	}

	documentedReasons := documentedReasonValues(doc)
	for _, reason := range documentedReasons {
		if _, found := allowed[reason]; found {
			continue
		}
		t.Fatalf("documented reason %q is outside the shared vocabulary", reason)
	}
}

func loadOpenAPIDoc(t *testing.T) *openapi3.T {
	t.Helper()

	specPath := filepath.Join("docs", "openapi.yaml")
	loader := &openapi3.Loader{IsExternalRefsAllowed: true}
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("load OpenAPI document (%s): %v", specPath, err)
	}
	return doc
}

func enumStrings(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			continue
		}
		out = append(out, text)
	}
	return out
}

func documentedReasonValues(doc *openapi3.T) []string {
	seen := map[string]struct{}{}
	for _, pathItem := range doc.Paths.Map() {
		for _, operation := range pathItem.Operations() {
			if operation.Responses == nil {
				continue
			}
			for _, responseRef := range operation.Responses.Map() {
				if responseRef == nil || responseRef.Value == nil {
					continue
				}
				for _, mediaRef := range responseRef.Value.Content {
					if mediaRef == nil {
						continue
					}
					recordReasonValue(seen, mediaRef.Example)
					for _, exampleRef := range mediaRef.Examples {
						if exampleRef == nil || exampleRef.Value == nil {
							continue
						}
						recordReasonValue(seen, exampleRef.Value.Value)
					}
				}
			}
		}
	}

	reasons := make([]string, 0, len(seen))
	for reason := range seen {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	return reasons
}

func recordReasonValue(seen map[string]struct{}, payload any) {
	exampleMap, ok := payload.(map[string]any)
	if !ok {
		return
	}
	reason, ok := exampleMap["reason"].(string)
	if !ok || reason == "" {
		return
	}
	seen[reason] = struct{}{}
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
