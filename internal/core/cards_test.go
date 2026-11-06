package core

import (
	"reflect"
	"testing"

	"github.com/knadh/listmonk/models"
)

func TestNormalizeCardCaptcha(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"empty inherits", "", models.CardCaptchaInherit, true},
		{"inherit", " inherit ", models.CardCaptchaInherit, true},
		{"on", "on", models.CardCaptchaOn, true},
		{"off", "off", models.CardCaptchaOff, true},
		{"invalid", "sometimes", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := normalizeCardCaptcha(tc.in)
			if got != tc.want || ok != tc.ok {
				t.Errorf("normalizeCardCaptcha(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

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
