package core

import (
	"reflect"
	"testing"
)

func TestNormalizeCardListIDs(t *testing.T) {
	cases := []struct {
		name string
		in   []int
		want []int
	}{
		{"nil stays nil", nil, nil},
		{"empty stays empty", []int{}, []int{}},
		{"keeps order", []int{3, 1, 2}, []int{3, 1, 2}},
		{"drops duplicates", []int{2, 2, 1, 2}, []int{2, 1}},
		{"drops non-positive", []int{0, -1, 4}, []int{4}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeCardListIDs(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("normalizeCardListIDs(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
