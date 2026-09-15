package generate

import (
	"testing"

	"4zreco/sliver/protobuf/clientpb"
)

func TestNameOfOutputFormatGoArchive(t *testing.T) {
	if got := nameOfOutputFormat(clientpb.OutputFormat_GO_ARCHIVE); got != "Go Archive" {
		t.Fatalf("nameOfOutputFormat(GO_ARCHIVE) = %q, want %q", got, "Go Archive")
	}
}
