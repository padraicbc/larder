package api

import (
	"net/http"

	"github.com/padraicbc/larder/internal/store"
)

func (s *Server) exportBackup(w http.ResponseWriter, r *http.Request) {
	backup, err := s.Store.ExportBackup(userID(r), s.Now())
	if err != nil {
		storeErr(w, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="larder-backup-`+s.Now().Format("2006-01-02")+`.json"`)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, backup)
}

func (s *Server) importBackup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, store.MaxBackupBytes)
	backup, err := store.ReadBackup(r.Body)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if r.URL.Path == "/api/backup/preview" {
		writeJSON(w, 200, backup.Summary())
		return
	}
	if err = s.Store.RestoreBackup(userID(r), backup); err != nil {
		storeErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, backup.Summary())
}
