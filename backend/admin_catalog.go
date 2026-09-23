package main

import (
	"bingo/bingo"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

var errCatalogConflict = errors.New("別の編集が保存されています。編集内容を控えてから再読み込みしてください")

func catalogRevision(contents []byte) string {
	hash := sha256.Sum256(contents)
	return hex.EncodeToString(hash[:])
}

func readCatalog(path string) (catalogDocument, []byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return catalogDocument{}, nil, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, adminMaxBody+1))
	if err != nil {
		return catalogDocument{}, nil, err
	}
	if len(contents) > adminMaxBody {
		return catalogDocument{}, nil, fmt.Errorf("お題ファイルが大きすぎます")
	}
	var data bingo.BingoData
	if err := json.Unmarshal(contents, &data); err != nil {
		return catalogDocument{}, nil, err
	}
	if err := bingo.ValidateData(data); err != nil {
		return catalogDocument{}, nil, err
	}
	return catalogDocument{Goals: data.Goals, RouteAreaTimes: data.RouteAreaTimes, BowserRoutes: data.BowserRoutes, Revision: catalogRevision(contents)}, contents, nil
}

func (c *catalogStore) load() (catalogDocument, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	document, _, err := readCatalog(c.path)
	return document, err
}

func (c *catalogStore) save(next catalogDocument) (catalogDocument, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := bingo.ValidateData(bingo.BingoData{Goals: next.Goals, RouteAreaTimes: next.RouteAreaTimes, BowserRoutes: next.BowserRoutes}); err != nil {
		return catalogDocument{}, err
	}
	current, previous, err := readCatalog(c.path)
	if err != nil {
		return catalogDocument{}, err
	}
	if next.Revision == "" || next.Revision != current.Revision {
		return catalogDocument{}, errCatalogConflict
	}
	contents, err := json.MarshalIndent(bingo.BingoData{Goals: next.Goals, RouteAreaTimes: next.RouteAreaTimes, BowserRoutes: next.BowserRoutes}, "", "  ")
	if err != nil {
		return catalogDocument{}, err
	}
	contents = append(contents, '\n')
	if len(contents) > adminMaxBody {
		return catalogDocument{}, fmt.Errorf("お題ファイルが大きすぎます")
	}
	backupDir := filepath.Join(filepath.Dir(c.path), ".bingo-backups")
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return catalogDocument{}, err
	}
	backup := filepath.Join(backupDir, time.Now().UTC().Format("20060102T150405Z")+"-"+uuid.NewString()+".json")
	if err := os.WriteFile(backup, previous, 0600); err != nil {
		return catalogDocument{}, err
	}
	if err := replaceCatalogFile(c.path, contents); err != nil {
		return catalogDocument{}, err
	}
	// Existing room cards are value snapshots; only future card generation changes.
	if err := bingo.ReplaceData(bingo.BingoData{Goals: next.Goals, RouteAreaTimes: next.RouteAreaTimes, BowserRoutes: next.BowserRoutes}); err != nil {
		return catalogDocument{}, err
	}
	next.Revision = catalogRevision(contents)
	return next, nil
}

func replaceCatalogFile(path string, contents []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".bingo-pending-")
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(contents); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	// Same-directory replacement keeps readers from observing a partially written file.
	// Failed replacements retain the pending file for recovery.
	return os.Rename(file.Name(), path)
}

func (s *server) adminGoals(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.currentAdmin(r); !ok {
		writeError(w, 401, "管理者としてDiscordでログインしてください")
		return
	}
	document, err := s.admin.catalog.load()
	if err != nil {
		writeError(w, 500, "お題ファイルを読み込めません。サーバーの保存先設定を確認してください")
		return
	}
	writeJSON(w, 200, document)
}

func (s *server) saveAdminGoals(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.currentAdmin(r); !ok {
		writeError(w, 401, "管理者としてDiscordでログインしてください")
		return
	}
	var next catalogDocument
	if !decodeLimit(w, r, &next, adminMaxBody) {
		return
	}
	if err := bingo.ValidateData(bingo.BingoData{Goals: next.Goals, RouteAreaTimes: next.RouteAreaTimes, BowserRoutes: next.BowserRoutes}); err != nil {
		writeError(w, 400, "お題データが不正です: "+err.Error())
		return
	}
	document, err := s.admin.catalog.save(next)
	if errors.Is(err, errCatalogConflict) {
		writeError(w, 409, err.Error())
		return
	}
	if err != nil {
		writeError(w, 500, "保存できません。保存先ディレクトリの書き込み権限と空き容量を確認してください")
		return
	}
	writeJSON(w, 200, document)
}
