package types

import "testing"

func TestPluginArgsBasic(t *testing.T) {
	pairs := []string{"arg0=ref", "output=/tmp/file", "insecure=true", "platform=linux/amd64", "platform=windows/amd64", "flag="}
	args := NewPluginArgs(pairs)

	if !args.Has("arg0") {
		t.Fatalf("expected positional entry")
	}

	if val, ok := args.Positional(0); !ok || val != "ref" {
		t.Fatalf("expected positional ref, got %q, %v", val, ok)
	}

	if val, ok := args.First("output"); !ok || val != "/tmp/file" {
		t.Fatalf("expected output value, got %q", val)
	}

	if count := args.Count("platform"); count != 2 {
		t.Fatalf("expected two platform entries, got %d", count)
	}

	list := args.All("platform")
	if len(list) != 2 || list[0] != "linux/amd64" || list[1] != "windows/amd64" {
		t.Fatalf("unexpected platform list: %#v", list)
	}

	if b, ok := args.Bool("insecure"); !ok || !b {
		t.Fatalf("expected insecure true, got %v, %v", b, ok)
	}

	if b, ok := args.Bool("flag"); !ok || !b {
		t.Fatalf("expected flag default true when empty, got %v, %v", b, ok)
	}
}

func TestPluginArgsBoolAny(t *testing.T) {
	pairs := []string{"h=true", "help=false", "o=path"}
	args := NewPluginArgs(pairs)

	if b, ok := args.BoolAny("help", "h"); !ok || b {
		t.Fatalf("expected BoolAny to favour first key value, got %v, %v", b, ok)
	}
}

func TestPluginArgsMissing(t *testing.T) {
	args := NewPluginArgs(nil)

	if _, ok := args.First("missing"); ok {
		t.Fatalf("expected missing key to be absent")
	}

	if _, ok := args.Positional(0); ok {
		t.Fatalf("expected no positional")
	}
}
