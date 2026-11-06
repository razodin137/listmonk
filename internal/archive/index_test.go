package archive

import "testing"

func TestRenderIndexListsPostsInOrder(t *testing.T) {
	got := RenderIndex([]IndexEntry{
		{Slug: "hello-world", Title: "Hello: World", Date: "2026-11-06T10:30:00Z", File: "hello-world.md"},
		{Slug: "a-b", Title: "A | B\nsecond line", File: "a-b.md"},
	})

	want := "# Archive index\n\n" +
		"| Slug | Title | Date | File |\n" +
		"| --- | --- | --- | --- |\n" +
		"| hello-world | Hello: World | 2026-11-06T10:30:00Z | hello-world.md |\n" +
		"| a-b | A \\| B second line |  | a-b.md |\n"

	if got != want {
		t.Fatalf("RenderIndex() = %q, want %q", got, want)
	}
}

func TestRenderIndexWithNoPosts(t *testing.T) {
	got := RenderIndex(nil)

	want := "# Archive index\n\n" +
		"| Slug | Title | Date | File |\n" +
		"| --- | --- | --- | --- |\n"

	if got != want {
		t.Fatalf("RenderIndex() = %q, want %q", got, want)
	}
}
