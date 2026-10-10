package args

import (
	"reflect"
	"testing"
)

func TestParseReturnsDefaults(t *testing.T) {
	result := Parse(nil, "/cwd")
	if !result.OK {
		t.Fatalf("Parse returned error: %v", result.Err)
	}
	if result.Options.MaxDepth != 3 {
		t.Fatalf("expected MaxDepth 3, got %d", result.Options.MaxDepth)
	}
	if !reflect.DeepEqual(result.Options.Dirs, []string{"/cwd"}) {
		t.Fatalf("expected Dirs [/cwd], got %v", result.Options.Dirs)
	}
	if result.Options.Verbose || result.Options.NoFetch || result.Options.Sync || result.Options.NonInteractive || result.Options.Help || result.Options.Version {
		t.Fatalf("expected boolean flags to default false: %+v", result.Options)
	}
}

func TestParseSupportsFlags(t *testing.T) {
	result := Parse([]string{"--verbose", "--no-fetch", "--sync", "--non-interactive", "--help", "--version"}, "/cwd")
	if !result.OK {
		t.Fatalf("Parse returned error: %v", result.Err)
	}
	if !result.Options.Verbose || !result.Options.NoFetch || !result.Options.Sync || !result.Options.NonInteractive || !result.Options.Help || !result.Options.Version {
		t.Fatalf("expected all flags true: %+v", result.Options)
	}
}

func TestParseSupportsMaxDepth(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want int
	}{
		{name: "separate value", argv: []string{"--max-depth", "5"}, want: 5},
		{name: "equals value", argv: []string{"--max-depth=7"}, want: 7},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Parse(test.argv, "/cwd")
			if !result.OK {
				t.Fatalf("Parse returned error: %v", result.Err)
			}
			if result.Options.MaxDepth != test.want {
				t.Fatalf("expected MaxDepth %d, got %d", test.want, result.Options.MaxDepth)
			}
		})
	}
}

func TestParseRejectsInvalidMaxDepth(t *testing.T) {
	for _, argv := range [][]string{{"--max-depth", "0"}, {"--max-depth", "abc"}, {"--max-depth"}} {
		if Parse(argv, "/cwd").OK {
			t.Fatalf("expected Parse(%v) to fail", argv)
		}
	}
}

func TestParseSupportsDir(t *testing.T) {
	tests := []struct {
		argv []string
		want []string
	}{
		{argv: []string{"--dir", "/some/path"}, want: []string{"/some/path"}},
		{argv: []string{"--dir=/another/path"}, want: []string{"/another/path"}},
		{argv: []string{"--dir", "/one", "--dir", "/two"}, want: []string{"/one", "/two"}},
		{argv: []string{"--dir=/one", "--no-fetch", "--dir", "two", "--max-depth", "2"}, want: []string{"/one", "two"}},
	}

	for _, test := range tests {
		result := Parse(test.argv, "/cwd")
		if !result.OK {
			t.Fatalf("Parse returned error: %v", result.Err)
		}
		if !reflect.DeepEqual(result.Options.Dirs, test.want) {
			t.Fatalf("expected Dirs %v, got %v", test.want, result.Options.Dirs)
		}
	}
}

func TestParseRejectsMissingDir(t *testing.T) {
	for _, argv := range [][]string{{"--dir"}, {"--dir", "--verbose"}, {"--dir="}, {"--dir", "/one", "--dir"}, {"--dir=/one", "--dir="}} {
		if Parse(argv, "/cwd").OK {
			t.Fatalf("expected Parse(%v) to fail", argv)
		}
	}
}

func TestParseRejectsUnknownArgs(t *testing.T) {
	for _, argv := range [][]string{{"--raw"}, {"--all"}} {
		if Parse(argv, "/cwd").OK {
			t.Fatalf("expected Parse(%v) to fail", argv)
		}
	}
}
