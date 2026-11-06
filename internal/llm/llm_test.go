package llm

import (
	"context"
	"testing"
)

type stub struct{ out string }

func (s stub) Model() string { return "stub" }
func (s stub) Complete(context.Context, Request) (string, error) {
	return s.out, nil
}

func TestClassify(t *testing.T) {
	opts := []Option{{ID: "POD_A", Description: "a"}, {ID: "POD_B", Description: "b"}}
	if id, err := Classify(context.Background(), stub{"  `pod_b`\n"}, "q", opts); err != nil || id != "POD_B" {
		t.Errorf("got %q, %v", id, err)
	}
	if _, err := Classify(context.Background(), stub{"NONE"}, "q", opts); err == nil {
		t.Error("NONE must be an error so the caller falls back")
	}
	if _, err := Classify(context.Background(), stub{"POD_A"}, "q", nil); err == nil {
		t.Error("empty options must fail")
	}
}
