package main

import (
	"bingo/bingo"
	"encoding/json"
	"errors"
	"maps"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestCatalogSavePersistsBacksUpAndKeepsExistingCards(t *testing.T) {
	s := testAdminServer(t)
	t.Cleanup(func() {
		if err := bingo.InitData("bingo.json"); err != nil {
			t.Error(err)
		}
	})
	host, _ := createTestRoom(t, s, "line")
	before, err := s.admin.catalog.load()
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(s.admin.catalog.path)
	if err != nil {
		t.Fatal(err)
	}
	next := catalogDocument{Goals: append([]bingo.BingoGoal(nil), before.Goals...), Revision: before.Revision}
	next.Goals[0].Name = "管理画面で更新したお題"
	next.RouteAreaTimes = maps.Clone(before.RouteAreaTimes)
	if next.RouteAreaTimes == nil {
		next.RouteAreaTimes = make(map[string]int)
	}
	next.RouteAreaTimes["shared"] = 1
	next.Goals[0].RouteAreas = []string{"shared"}
	next.Goals[0].ConflictGroups = []string{"nested"}
	next.BowserRoutes = []bingo.FinishRoute{{Name: "star", TimeMin: 6, RouteAreas: []string{"shared"}}}
	after, err := s.admin.catalog.save(next)
	if err != nil || after.Revision == before.Revision {
		t.Fatal(after.Revision, err)
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(s.admin.catalog.path), ".bingo-backups", "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	backup, err := os.ReadFile(files[0])
	if err != nil || string(backup) != string(original) {
		t.Fatal("incorrect backup", err)
	}
	reopened := &catalogStore{path: s.admin.catalog.path}
	persisted, err := reopened.load()
	if err != nil || !reflect.DeepEqual(persisted, after) {
		t.Fatal("save did not persist", err)
	}
	if bingo.GetBingoData().Goals[0].Name != next.Goals[0].Name {
		t.Fatal("live catalog not updated")
	}
	if bingo.GetBingoData().RouteAreaTimes["shared"] != 1 || bingo.GetBingoData().Goals[0].RouteAreas[0] != "shared" {
		t.Fatal("live route data not updated")
	}
	game, _ := s.rooms.GetRoom(host.Room.ID)
	if !reflect.DeepEqual(game.GetRoomStatus().Card, host.Room.Card) {
		t.Fatal("existing card changed")
	}
	next.BowserRoutes[0].RouteAreas[0] = "mutated"
	if bingo.GetBingoData().BowserRoutes[0].RouteAreas[0] != "shared" {
		t.Fatal("finish route alias")
	}
	next.Goals[0].Tags[0] = "area:changed"
	next.RouteAreaTimes["shared"] = 9
	next.Goals[0].RouteAreas[0] = "changed"
	if bingo.GetBingoData().RouteAreaTimes["shared"] != 1 || bingo.GetBingoData().Goals[0].RouteAreas[0] != "shared" {
		t.Fatal("live routes alias request")
	}
	if bingo.GetBingoData().Goals[0].Tags[0] == "area:changed" {
		t.Fatal("live data aliases request")
	}
	if _, err := reopened.save(before); !errors.Is(err, errCatalogConflict) {
		t.Fatal("stale edit accepted", err)
	}
}

func TestCatalogRejectsInvalidEditsAndConcurrentOverwrites(t *testing.T) {
	s := testAdminServer(t)
	t.Cleanup(func() { _ = bingo.InitData("bingo.json") })
	doc, err := s.admin.catalog.load()
	if err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(s.admin.catalog.path)
	if _, err := s.admin.catalog.save(catalogDocument{Goals: doc.Goals[:24], Revision: doc.Revision}); err == nil {
		t.Fatal("invalid catalog saved")
	}
	if _, err := s.admin.catalog.save(catalogDocument{Goals: doc.Goals, RouteAreaTimes: doc.RouteAreaTimes}); !errors.Is(err, errCatalogConflict) {
		t.Fatal("missing revision accepted")
	}
	unchanged, _ := os.ReadFile(s.admin.catalog.path)
	if string(unchanged) != string(original) {
		t.Fatal("rejected edit changed file")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			next := catalogDocument{Revision: doc.Revision, Goals: append([]bingo.BingoGoal(nil), doc.Goals...), RouteAreaTimes: maps.Clone(doc.RouteAreaTimes)}
			next.Goals[i].TimeMin++
			_, err := s.admin.catalog.save(next)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, errCatalogConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
}

func TestCatalogReadAndWriteFailures(t *testing.T) {
	s := testAdminServer(t)
	doc, err := s.admin.catalog.load()
	if err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(s.admin.catalog.path)
	if err := os.WriteFile(filepath.Join(filepath.Dir(s.admin.catalog.path), ".bingo-backups"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	doc.Goals[0].TimeMin++
	if _, err := s.admin.catalog.save(doc); err == nil {
		t.Fatal("backup failure ignored")
	}
	unchanged, _ := os.ReadFile(s.admin.catalog.path)
	if string(unchanged) != string(original) {
		t.Fatal("backup failure changed original")
	}
	if err := replaceCatalogFile(filepath.Join(t.TempDir(), "missing", "data.json"), []byte("{}")); err == nil {
		t.Fatal("missing directory ignored")
	}
	if err := replaceCatalogFile(t.TempDir(), []byte("{}")); err == nil {
		t.Fatal("directory replaced")
	}
	for _, contents := range []string{"{", `{"goals":[]}`, strings.Repeat(" ", adminMaxBody+1)} {
		path := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readCatalog(path); err == nil {
			t.Fatal("invalid catalog read")
		}
	}
	if _, _, err := readCatalog(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing file accepted")
	}
}

func TestAdminCatalogAPIValidationAndCSRF(t *testing.T) {
	s := testAdminServer(t)
	t.Cleanup(func() { _ = bingo.InitData("bingo.json") })
	cookie := loginAdmin(t, s)
	w := request(t, s, "GET", "/api/admin/goals", nil, cookie, 200)
	var doc catalogDocument
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	request(t, s, "PUT", "/api/admin/goals", doc, nil, 401)
	r := httptest.NewRequest("PUT", "/api/admin/goals", strings.NewReader(`{}`))
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-site form accepted")
	}
	request(t, s, "PUT", "/api/admin/goals", map[string]any{"goals": doc.Goals, "revision": doc.Revision, "extra": true}, cookie, 400)
	request(t, s, "PUT", "/api/admin/goals", catalogDocument{Goals: doc.Goals[:24], Revision: doc.Revision}, cookie, 400)
	doc.Goals[0].TimeMin++
	request(t, s, "PUT", "/api/admin/goals", doc, cookie, 200)
	request(t, s, "PUT", "/api/admin/goals", doc, cookie, 409)
	s.admin.catalog.path = filepath.Join(t.TempDir(), "missing.json")
	request(t, s, "GET", "/api/admin/goals", nil, cookie, 500)
	request(t, s, "PUT", "/api/admin/goals", doc, cookie, 500)
}
