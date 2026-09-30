package main

import (
	"flag"
	"reflect"
	"testing"
)

func TestReorderArgsPreservesBooleanFlagsAndLateGraph(t *testing.T) {
	for _, args := range [][]string{
		{"--json", "How does AutoRepair work?", "--graph", "selected.json"},
		{"How does AutoRepair work?", "--json", "--graph", "selected.json"},
		{"--json=true", "How does AutoRepair work?", "--graph=selected.json"},
	} {
		fs := flag.NewFlagSet("evidence", flag.ContinueOnError)
		graph := fs.String("graph", "default.json", "")
		json := fs.Bool("json", false, "")
		if err := fs.Parse(reorderArgs(fs, args)); err != nil {
			t.Fatal(err)
		}
		if *graph != "selected.json" || !*json || !reflect.DeepEqual(fs.Args(), []string{"How does AutoRepair work?"}) {
			t.Fatalf("args=%v: graph=%s json=%v positional=%v", args, *graph, *json, fs.Args())
		}
	}
}

func TestReorderArgsPreservesTerminatorAndFlagValues(t *testing.T) {
	fs := flag.NewFlagSet("evidence", flag.ContinueOnError)
	question := fs.String("question", "", "")
	if err := fs.Parse(reorderArgs(fs, []string{"--question", "-literal question", "--", "--graph", "a.json"})); err != nil {
		t.Fatal(err)
	}
	if *question != "-literal question" || !reflect.DeepEqual(fs.Args(), []string{"--graph", "a.json"}) {
		t.Fatalf("flag values or terminator lost: question=%q args=%v", *question, fs.Args())
	}
}
