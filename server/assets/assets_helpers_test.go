package assets

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"4zreco/var/sliver/protobuf"
)

func TestSetupGoPathCopiesSliverpbCapabilities(t *testing.T) {
	destination := t.TempDir()
	if err := SetupGoPath(destination, false); err != nil {
		t.Fatalf("SetupGoPath() error = %v", err)
	}

	want, err := protobufs.FS.ReadFile("sliverpb/capabilities.go")
	if err != nil {
		t.Fatalf("read embedded capabilities.go: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(destination, "protobuf", "sliverpb", "capabilities.go"))
	if err != nil {
		t.Fatalf("read extracted capabilities.go: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("extracted capabilities.go does not match the embedded source")
	}
}
