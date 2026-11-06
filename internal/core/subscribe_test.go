package core

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v5"
)

type fakeSubscriberStore struct {
	nextID  int
	subs    map[string]models.Subscriber
	labels  map[int]map[string]struct{}
	updates []fakeSubscriberUpdate
}

type fakeSubscriberUpdate struct {
	sub         models.Subscriber
	uuids       []string
	deleteLists bool
}

func newFakeSubscriberStore() *fakeSubscriberStore {
	return &fakeSubscriberStore{
		subs:   map[string]models.Subscriber{},
		labels: map[int]map[string]struct{}{},
	}
}

func (s *fakeSubscriberStore) InsertSubscriber(sub models.Subscriber, _ []int, listUUIDs []string, _, _ bool) (models.Subscriber, bool, error) {
	if _, ok := s.subs[sub.Email]; ok {
		return models.Subscriber{}, false, echo.NewHTTPError(http.StatusConflict, "subscriber exists")
	}

	s.nextID++
	sub.ID = s.nextID
	s.subs[sub.Email] = sub
	s.addLabels(sub.ID, listUUIDs)

	return sub, false, nil
}

func (s *fakeSubscriberStore) GetSubscriber(id int, uuid, email string) (models.Subscriber, error) {
	for _, sub := range s.subs {
		if id > 0 && sub.ID == id {
			return sub, nil
		}
		if id == 0 && uuid == "" && sub.Email == email {
			return sub, nil
		}
	}

	return models.Subscriber{}, echo.NewHTTPError(http.StatusBadRequest, "subscriber not found")
}

func (s *fakeSubscriberStore) UpdateSubscriberWithLists(id int, sub models.Subscriber, _ []int, listUUIDs []string, _, deleteLists, _ bool, _ []int, _ bool) (models.Subscriber, bool, error) {
	s.updates = append(s.updates, fakeSubscriberUpdate{
		sub:         sub,
		uuids:       append([]string(nil), listUUIDs...),
		deleteLists: deleteLists,
	})

	existing, err := s.GetSubscriber(id, "", "")
	if err != nil {
		return models.Subscriber{}, false, err
	}

	if deleteLists {
		s.labels[id] = map[string]struct{}{}
	}
	if sub.Email != "" {
		delete(s.subs, existing.Email)
		existing.Email = sub.Email
	}
	if sub.Name != "" {
		existing.Name = sub.Name
	}
	if sub.Status != "" {
		existing.Status = sub.Status
	}

	s.subs[existing.Email] = existing
	s.addLabels(existing.ID, listUUIDs)

	return existing, false, nil
}

func (s *fakeSubscriberStore) addLabels(id int, listUUIDs []string) {
	if s.labels[id] == nil {
		s.labels[id] = map[string]struct{}{}
	}

	for _, u := range listUUIDs {
		s.labels[id][u] = struct{}{}
	}
}

func TestSubscribeTwiceAddsListsToExistingSubscriber(t *testing.T) {
	store := newFakeSubscriberStore()

	sub := models.Subscriber{
		Name:   "Ada",
		Email:  "ada@example.com",
		Status: models.SubscriberStatusEnabled,
	}

	if _, err := subscribeSubscriber(store, sub, []string{"news"}); err != nil {
		t.Fatalf("first signup: %v", err)
	}

	existing := store.subs[sub.Email]
	store.addLabels(existing.ID, []string{"digest"})

	hasOptin, err := subscribeSubscriber(store, models.Subscriber{
		Name:   "Different",
		Email:  sub.Email,
		Status: models.SubscriberStatusBlockListed,
	}, []string{"news", "events"})
	if err != nil {
		t.Fatalf("second signup: %v", err)
	}
	if hasOptin {
		t.Error("second signup asked for opt-in, want none")
	}

	existing = store.subs[sub.Email]
	if got, want := existing.Name, "Ada"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
	if got, want := existing.Status, models.SubscriberStatusEnabled; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}

	wantLabels := map[string]struct{}{
		"news":   {},
		"digest": {},
		"events": {},
	}
	if got := store.labels[existing.ID]; !reflect.DeepEqual(got, wantLabels) {
		t.Errorf("labels = %v, want %v", got, wantLabels)
	}

	if len(store.updates) != 1 {
		t.Fatalf("update calls = %d, want 1", len(store.updates))
	}
	if store.updates[0].deleteLists {
		t.Error("second signup deleted existing labels, want them kept")
	}
	if got, want := store.updates[0].uuids, []string{"news", "events"}; !reflect.DeepEqual(got, want) {
		t.Errorf("updated labels = %v, want %v", got, want)
	}
}
