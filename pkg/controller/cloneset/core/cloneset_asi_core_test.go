package core

import "testing"

func TestRexp(t *testing.T) {
	successTests := []string{
		"/spec/containers/1/image",
		"/spec/containers/0/env/3/value",
		"/spec/containers/10/lifecycle/preStop",
		"/spec/containers/5/args/0",
	}
	failureTests := []string{
		"/spec/containers/1/name",
		"/spec/containers/0/volumeMounts/3",
		"/spec/initContainers/1/image",
		"/spec/volumes/3",
	}

	for _, s := range successTests {
		if !inPlaceUpdateTemplateSpecPatchASIRexp.MatchString(s) {
			t.Fatalf("expected %s success, but got not matched", s)
		}
	}
	for _, s := range failureTests {
		if inPlaceUpdateTemplateSpecPatchASIRexp.MatchString(s) {
			t.Fatalf("expected %s failure, but got matched", s)
		}
	}
}
