package catalog

import "testing"

func TestLoadEmbedded(t *testing.T) {
	root, err := LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Flavors) < 3 {
		t.Fatalf("expected flavors, got %d", len(root.Flavors))
	}
	if _, ok := root.FlavorByID("ascend-910b2-whole-card"); !ok {
		t.Fatal("missing 910b2 flavor")
	}
}
