package core

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/jmoiron/sqlx/types"
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
	preconfirm  bool
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

func (s *fakeSubscriberStore) UpdateSubscriberWithLists(id int, sub models.Subscriber, _ []int, listUUIDs []string, preconfirm, deleteLists, _ bool, _ []int, _ bool) (models.Subscriber, bool, error) {
	s.updates = append(s.updates, fakeSubscriberUpdate{
		sub:         sub,
		uuids:       append([]string(nil), listUUIDs...),
		preconfirm:  preconfirm,
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

func TestPendingOptinListIDsOnlyIncludesRequestedLists(t *testing.T) {
	sub := models.Subscriber{Lists: types.JSONText(`[
		{"id": 1, "uuid": "card-double", "optin": "double", "subscription_status": "unconfirmed"},
		{"id": 2, "uuid": "card-confirmed", "optin": "double", "subscription_status": "confirmed"},
		{"id": 3, "uuid": "other-double", "optin": "double", "subscription_status": "unconfirmed"},
		{"id": 4, "uuid": "card-single", "optin": "single", "subscription_status": "unconfirmed"}
	]`)}

	cases := []struct {
		name  string
		ids   []int
		uuids []string
		want  []int
	}{
		{"by uuid", nil, []string{"card-double", "card-confirmed", "card-single"}, []int{1}},
		{"by id", []int{1, 2}, nil, []int{1}},
		{"all pending", nil, nil, []int{1, 3}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pendingOptinListIDs(sub, tc.ids, tc.uuids); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("pending = %v, want %v", got, tc.want)
			}
		})
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

// setSubscriberLists stores the raw list JSON on a subscriber already in the
// fake store, the way GetSubscriber would return it from the database.
func setSubscriberLists(store *fakeSubscriberStore, email, lists string) {
	sub := store.subs[email]
	sub.Lists = types.JSONText(lists)
	store.subs[email] = sub
}

func TestSubscribeConfirmedMemberSkipsOptin(t *testing.T) {
	store := newFakeSubscriberStore()
	email := "ada@example.com"

	if _, err := subscribeSubscriber(store, models.Subscriber{
		Name:   "Ada",
		Email:  email,
		Status: models.SubscriberStatusEnabled,
	}, []string{"news"}); err != nil {
		t.Fatalf("first signup: %v", err)
	}

	setSubscriberLists(store, email, `[{"uuid":"news","subscription_status":"confirmed"}]`)

	hasOptin, err := subscribeSubscriber(store, models.Subscriber{
		Name:   "Ada",
		Email:  email,
		Status: models.SubscriberStatusEnabled,
	}, []string{"news"})
	if err != nil {
		t.Fatalf("second signup: %v", err)
	}
	if hasOptin {
		t.Error("confirmed member asked for opt-in, want none")
	}

	if len(store.updates) != 1 {
		t.Fatalf("update calls = %d, want 1", len(store.updates))
	}
	if !store.updates[0].preconfirm {
		t.Error("confirmed member was not preconfirmed, want immediate subscription")
	}
}

func TestSubscribeResultSplitsNewAndExistingMembers(t *testing.T) {
	store := newFakeSubscriberStore()
	email := "ada@example.com"

	newRes, err := subscribeSubscriberWithResult(store, models.Subscriber{
		Name:   "Ada",
		Email:  email,
		Status: models.SubscriberStatusEnabled,
	}, []string{"news"})
	if err != nil {
		t.Fatalf("first signup: %v", err)
	}
	if !newRes.New {
		t.Error("first signup New = false, want true")
	}
	if !newRes.GainedLabels {
		t.Error("first signup GainedLabels = false, want true")
	}
	if newRes.Subscriber.ID == 0 {
		t.Error("first signup returned no subscriber ID")
	}

	setSubscriberLists(store, email, `[{"uuid":"news","subscription_status":"confirmed"}]`)

	gainedRes, err := subscribeSubscriberWithResult(store, models.Subscriber{
		Name:   "Ada",
		Email:  email,
		Status: models.SubscriberStatusEnabled,
	}, []string{"news", "events"})
	if err != nil {
		t.Fatalf("second signup: %v", err)
	}
	if gainedRes.New {
		t.Error("existing signup New = true, want false")
	}
	if !gainedRes.GainedLabels {
		t.Error("existing signup GainedLabels = false, want true")
	}

	setSubscriberLists(store, email, `[
		{"uuid":"news","subscription_status":"confirmed"},
		{"uuid":"events","subscription_status":"confirmed"}
	]`)

	sameRes, err := subscribeSubscriberWithResult(store, models.Subscriber{
		Name:   "Ada",
		Email:  email,
		Status: models.SubscriberStatusEnabled,
	}, []string{"news", "events"})
	if err != nil {
		t.Fatalf("third signup: %v", err)
	}
	if sameRes.New {
		t.Error("existing signup New = true, want false")
	}
	if sameRes.GainedLabels {
		t.Error("existing signup with every label GainedLabels = true, want false")
	}
}

func TestSubscriberGainedLabels(t *testing.T) {
	cases := []struct {
		name   string
		status string
		lists  string
		want   bool
	}{
		{"no subscriptions", models.SubscriberStatusEnabled, `[]`, true},
		{"new label", models.SubscriberStatusEnabled, `[{"uuid":"other","subscription_status":"confirmed"}]`, true},
		{"already confirmed", models.SubscriberStatusEnabled, `[{"uuid":"news","subscription_status":"confirmed"}]`, false},
		{"pending confirmation", models.SubscriberStatusEnabled, `[{"uuid":"news","subscription_status":"unconfirmed"}]`, false},
		{"unsubscribed", models.SubscriberStatusEnabled, `[{"uuid":"news","subscription_status":"unsubscribed"}]`, true},
		{"blocklisted", models.SubscriberStatusBlockListed, `[{"uuid":"news","subscription_status":"unsubscribed"}]`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub := models.Subscriber{Status: tc.status, Lists: types.JSONText(tc.lists)}
			if got := subscriberGainedLabels(sub, []string{"news"}); got != tc.want {
				t.Errorf("subscriberGainedLabels() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSubscribeUnconfirmedOrDisabledMemberStillOptsIn(t *testing.T) {
	cases := []struct {
		name   string
		status string
		lists  string
	}{
		{"unconfirmed label", models.SubscriberStatusEnabled, `[{"uuid":"news","subscription_status":"unconfirmed"}]`},
		{"label not subscribed", models.SubscriberStatusEnabled, `[{"uuid":"other","subscription_status":"confirmed"}]`},
		{"disabled subscriber", models.SubscriberStatusDisabled, `[{"uuid":"news","subscription_status":"confirmed"}]`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeSubscriberStore()
			email := "ada@example.com"

			if _, err := subscribeSubscriber(store, models.Subscriber{
				Name:   "Ada",
				Email:  email,
				Status: models.SubscriberStatusEnabled,
			}, []string{"news"}); err != nil {
				t.Fatalf("first signup: %v", err)
			}

			sub := store.subs[email]
			sub.Status = tc.status
			store.subs[email] = sub
			setSubscriberLists(store, email, tc.lists)

			if _, err := subscribeSubscriber(store, models.Subscriber{
				Name:   "Ada",
				Email:  email,
				Status: models.SubscriberStatusEnabled,
			}, []string{"news"}); err != nil {
				t.Fatalf("second signup: %v", err)
			}

			if len(store.updates) != 1 {
				t.Fatalf("update calls = %d, want 1", len(store.updates))
			}
			if store.updates[0].preconfirm {
				t.Error("member was preconfirmed, want opt-in")
			}
		})
	}
}
