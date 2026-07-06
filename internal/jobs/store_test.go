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
		UserID: "user-a",
		Title:  "测试产品",
		MDPath: "/tmp/product.md",
		MDName: "product.md",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.GetProduct("user-a", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "测试产品" || got.MDName != "product.md" {
		t.Fatalf("unexpected product: %+v", got)
	}

	products, err := store.ListProducts("user-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(products))
	}
	otherProducts, err := store.ListProducts("user-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(otherProducts) != 0 {
		t.Fatalf("expected product isolation, got %d products", len(otherProducts))
	}
}

func TestStoreProductChunks(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	product, err := store.CreateProduct(CreateProductInput{
		UserID: "user-a",
		Title:  "测试产品",
		MDPath: "/tmp/product.md",
		MDName: "product.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceProductChunks(product.ID, []ProductChunkInput{
		{
			ChunkIndex:     0,
			Heading:        "玩法",
			Content:        "三消排序",
			Embedding:      []float64{0.1, 0.2, 0.3},
			EmbeddingModel: "text-embedding-v4",
			EmbeddingDim:   3,
		},
	}); err != nil {
		t.Fatal(err)
	}

	chunks, err := store.ListProductChunks(product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Heading != "玩法" || chunks[0].EmbeddingDim != 3 || len(chunks[0].Embedding) != 3 {
		t.Fatalf("unexpected chunk: %+v", chunks[0])
	}
}

func TestStoreCreativeReports(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	product, err := store.CreateProduct(CreateProductInput{
		UserID: "user-a",
		Title:  "测试产品",
		MDPath: "/tmp/product.md",
		MDName: "product.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.CreateCreativeReport(CreateCreativeReportInput{
		UserID:           "user-a",
		ProductID:        product.ID,
		ProductTitle:     product.Title,
		SourceConfigJSON: `{"range":"30d"}`,
		ReportMarkdown:   "# 报告\n创意方向",
		ReportSummary:    "创意方向摘要",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetCreativeReport("user-a", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProductID != product.ID || got.ReportSummary != "创意方向摘要" {
		t.Fatalf("unexpected report: %+v", got)
	}
	reports, err := store.ListCreativeReports("user-a", product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	otherReports, err := store.ListCreativeReports("user-b", product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherReports) != 0 {
		t.Fatalf("expected report isolation, got %d reports", len(otherReports))
	}
}

func TestStoreModelRuntimeConfig(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := store.SaveModelSettings("user-a", ModelSettings{
		APIKey:   "sk-test",
		Endpoint: "https://example.com/api",
		Model:    "qwen-test",
	}); err != nil {
		t.Fatal(err)
	}

	settings, err := store.GetModelSettings("user-a")
	if err != nil {
		t.Fatal(err)
	}
	if settings.APIKey != "sk-test" || settings.Endpoint != "https://example.com/api" || settings.Model != "qwen-test" {
		t.Fatalf("unexpected model settings: %+v", settings)
	}
	if _, err := store.GetModelSettings("user-b"); err == nil {
		t.Fatal("expected user-b settings to be missing")
	}
}
