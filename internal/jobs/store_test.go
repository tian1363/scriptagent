package jobs

import (
	"path/filepath"
	"testing"
)

func TestStoreProducts(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	created, err := store.CreateProduct(CreateProductInput{
		Title:  "测试产品",
		MDPath: "/tmp/product.md",
		MDName: "product.md",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.GetProduct(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "测试产品" || got.MDName != "product.md" {
		t.Fatalf("unexpected product: %+v", got)
	}

	products, err := store.ListProducts()
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(products))
	}
}

func TestStoreModelRuntimeConfig(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := store.SaveModelSettings(ModelSettings{
		APIKey:   "sk-test",
		Endpoint: "https://example.com/api",
		Model:    "qwen-test",
	}); err != nil {
		t.Fatal(err)
	}

	runtime, err := store.GetModelRuntimeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if runtime.APIKey != "sk-test" || runtime.Endpoint != "https://example.com/api" || runtime.Model != "qwen-test" || runtime.Source != "user" {
		t.Fatalf("unexpected runtime config: %+v", runtime)
	}
}
