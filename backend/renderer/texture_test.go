package renderer

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestShopTypeAllowlist(t *testing.T) {
	for _, bad := range []string{"", "..", "../hat", "hat/..", "hat/../../etc", "HAT/../x", "all", "secret"} {
		if _, ok := cleanShopType(bad); ok {
			t.Errorf("cleanShopType(%q) accepted", bad)
		}
	}
	if got, ok := cleanShopType(" Hat "); !ok || got != "hat" {
		t.Fatalf("cleanShopType = %q, %v", got, ok)
	}
	keys := shopStoreKeys("gear", 7, ".obj")
	want := []string{"assets/uploads/shop/gear/7.obj", "static/uploads/shop/gear/7.obj", "uploads/shop/gear/7.obj"}
	if len(keys) != len(want) {
		t.Fatalf("shopStoreKeys = %v", keys)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("shopStoreKeys = %v", keys)
		}
	}
	if _, err := fetchTexture("../../etc", 1, ""); err == nil {
		t.Fatal("fetchTexture accepted a traversal type")
	}
	if _, err := fetchMesh("../../etc", 1); err == nil {
		t.Fatal("fetchMesh accepted a traversal type")
	}
}

func TestAssetMeshKeyRejectsTraversal(t *testing.T) {
	for _, bad := range []string{"../x", "1/../../x", "/etc/passwd", "1\\..\\x", "", "..", "1/../../../static/x"} {
		if key, err := assetMeshKey(bad); err == nil {
			t.Errorf("assetMeshKey(%q) = %q", bad, key)
		}
	}
	if key, err := assetMeshKey("12/abc.glb"); err != nil || key != "uploads/assets/12/abc.glb" {
		t.Fatalf("assetMeshKey = %q, %v", key, err)
	}
}

func TestMalformedMeshesDoNotPanic(t *testing.T) {
	dir := t.TempDir()
	obj := filepath.Join(dir, "bad.obj")
	if err := os.WriteFile(obj, []byte("v 1\nf 1 2 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := MeshTriangleCount(obj); err == nil && n > 0 {
		t.Fatalf("malformed OBJ produced %d triangles", n)
	}
	// A GLB whose chunk header claims 4 GiB must not be allocated.
	glb := make([]byte, 20)
	copy(glb[0:4], "glTF")
	binary.LittleEndian.PutUint32(glb[4:8], 2)
	binary.LittleEndian.PutUint32(glb[8:12], 20)
	binary.LittleEndian.PutUint32(glb[12:16], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(glb[16:20], 0x4E4F534A)
	path := filepath.Join(dir, "bomb.glb")
	if err := os.WriteFile(path, glb, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := MeshTriangleCount(path); err == nil {
		t.Fatal("oversized GLB chunk accepted")
	}
	if tex := extractGLBTexture(path); tex != nil {
		t.Fatal("oversized GLB chunk produced a texture")
	}
}
