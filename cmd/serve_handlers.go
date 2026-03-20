package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sheeppattern/zk/internal/model"
	"github.com/sheeppattern/zk/internal/store"
)

type serveHandler struct {
	store *store.Store
}

// apiNoteView mirrors output.noteView — includes Content which model.Note excludes via json:"-".
type apiNoteView struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	Layer     string         `json:"layer"`
	Tags      []string       `json:"tags"`
	Links     []model.Link   `json:"links"`
	Metadata  model.Metadata `json:"metadata"`
	ProjectID string         `json:"project_id,omitempty"`
}

func toAPIView(n *model.Note) apiNoteView {
	layer := n.Layer
	if layer == "" {
		layer = "concrete"
	}
	tags := n.Tags
	if tags == nil {
		tags = []string{}
	}
	links := n.Links
	if links == nil {
		links = []model.Link{}
	}
	return apiNoteView{
		ID:        n.ID,
		Title:     n.Title,
		Content:   n.Content,
		Layer:     layer,
		Tags:      tags,
		Links:     links,
		Metadata:  n.Metadata,
		ProjectID: n.ProjectID,
	}
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		debugf("json response encode error: %v", err)
	}
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		debugf("json error encode error: %v", err)
	}
}

// logNotesPartial calls ListNotesPartial and logs any parse errors via debugf.
func (h *serveHandler) logNotesPartial(projectID string) []*model.Note {
	notes, noteErrors := h.store.ListNotesPartial(projectID)
	for _, ne := range noteErrors {
		debugf("corrupted note %s: %v", ne.FilePath, ne.Err)
	}
	return notes
}

// GET /api/projects
func (h *serveHandler) handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonError(w, "method not allowed", 405)
		return
	}
	projects, err := h.store.ListProjects()
	if err != nil {
		debugf("list projects: %v", err)
		jsonError(w, "failed to list projects", 500)
		return
	}
	if projects == nil {
		projects = []*model.Project{}
	}

	type projectView struct {
		*model.Project
		NoteCount int `json:"note_count"`
		LinkCount int `json:"link_count"`
	}
	var views []projectView
	for _, p := range projects {
		notes := h.logNotesPartial(p.ID)
		lc := 0
		for _, n := range notes {
			lc += len(n.Links)
		}
		views = append(views, projectView{Project: p, NoteCount: len(notes), LinkCount: lc})
	}
	if views == nil {
		views = []projectView{}
	}
	jsonResponse(w, views)
}

// GET /api/notes?project=X
func (h *serveHandler) handleNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonError(w, "method not allowed", 405)
		return
	}
	project := r.URL.Query().Get("project")
	notes := h.logNotesPartial(project)
	views := make([]apiNoteView, 0, len(notes))
	for _, n := range notes {
		views = append(views, toAPIView(n))
	}
	jsonResponse(w, views)
}

// GET/PUT/DELETE /api/note?id=X&project=X
func (h *serveHandler) handleNote(w http.ResponseWriter, r *http.Request) {
	noteID := r.URL.Query().Get("id")
	projectID := r.URL.Query().Get("project")

	switch r.Method {
	case "GET":
		note, err := h.store.GetNote(projectID, noteID)
		if err != nil {
			debugf("get note %s: %v", noteID, err)
			jsonError(w, "note not found", 404)
			return
		}
		jsonResponse(w, toAPIView(note))

	case "PUT":
		var body struct {
			Title   *string  `json:"title"`
			Content *string  `json:"content"`
			Tags    []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid JSON body", 400)
			return
		}
		note, err := h.store.GetNote(projectID, noteID)
		if err != nil {
			debugf("get note for update %s: %v", noteID, err)
			jsonError(w, "note not found", 404)
			return
		}
		if body.Title != nil {
			note.Title = *body.Title
		}
		if body.Content != nil {
			note.Content = *body.Content
		}
		if body.Tags != nil {
			note.Tags = body.Tags
		}
		if err := h.store.UpdateNote(note); err != nil {
			debugf("update note %s: %v", noteID, err)
			jsonError(w, "failed to save note", 500)
			return
		}
		jsonResponse(w, toAPIView(note))

	case "DELETE":
		if err := h.store.DeleteNote(projectID, noteID); err != nil {
			debugf("delete note %s: %v", noteID, err)
			jsonError(w, "failed to delete note", 500)
			return
		}
		jsonResponse(w, map[string]bool{"ok": true})

	default:
		jsonError(w, "method not allowed", 405)
	}
}

// GET /api/search?q=X&project=X
func (h *serveHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonError(w, "method not allowed", 405)
		return
	}
	query := strings.ToLower(r.URL.Query().Get("q"))
	project := r.URL.Query().Get("project")
	if query == "" {
		jsonError(w, "missing query parameter 'q'", 400)
		return
	}

	notes := h.logNotesPartial(project)

	type scored struct {
		view  apiNoteView
		score int
	}
	var results []scored

	for _, n := range notes {
		score := 0
		if strings.Contains(strings.ToLower(n.Title), query) {
			score += 2
		}
		if strings.Contains(strings.ToLower(n.Content), query) {
			score++
		}
		for _, tag := range n.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				score++
			}
		}
		if score > 0 {
			results = append(results, scored{view: toAPIView(n), score: score})
		}
	}

	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	views := make([]apiNoteView, 0, len(results))
	for _, r := range results {
		views = append(views, r.view)
	}
	jsonResponse(w, views)
}

// GET /api/all-data
func (h *serveHandler) handleAllData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonError(w, "method not allowed", 405)
		return
	}

	projects, _ := h.store.ListProjects()
	if projects == nil {
		projects = []*model.Project{}
	}

	allNotes := make([]apiNoteView, 0)
	for _, p := range projects {
		notes := h.logNotesPartial(p.ID)
		for _, n := range notes {
			allNotes = append(allNotes, toAPIView(n))
		}
	}
	gNotes := h.logNotesPartial("")
	for _, n := range gNotes {
		allNotes = append(allNotes, toAPIView(n))
	}

	jsonResponse(w, map[string]interface{}{
		"projects": projects,
		"notes":    allNotes,
	})
}

// POST /api/tag  body: {note_id, project_id, tag, action: "add"|"remove"}
func (h *serveHandler) handleTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "method not allowed", 405)
		return
	}
	var body struct {
		NoteID    string `json:"note_id"`
		ProjectID string `json:"project_id"`
		Tag       string `json:"tag"`
		Action    string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid JSON body", 400)
		return
	}

	note, err := h.store.GetNote(body.ProjectID, body.NoteID)
	if err != nil {
		debugf("get note for tag %s: %v", body.NoteID, err)
		jsonError(w, "note not found", 404)
		return
	}

	switch body.Action {
	case "add":
		for _, t := range note.Tags {
			if t == body.Tag {
				jsonResponse(w, toAPIView(note))
				return
			}
		}
		note.Tags = append(note.Tags, body.Tag)
	case "remove":
		filtered := make([]string, 0, len(note.Tags))
		for _, t := range note.Tags {
			if t != body.Tag {
				filtered = append(filtered, t)
			}
		}
		note.Tags = filtered
	default:
		jsonError(w, fmt.Sprintf("invalid action %q, expected 'add' or 'remove'", body.Action), 400)
		return
	}

	if err := h.store.UpdateNote(note); err != nil {
		debugf("update note tags %s: %v", body.NoteID, err)
		jsonError(w, "failed to update tags", 500)
		return
	}
	jsonResponse(w, toAPIView(note))
}

// DELETE /api/link  body: {source, target, type, project, target_project}
func (h *serveHandler) handleLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		jsonError(w, "method not allowed", 405)
		return
	}
	var body struct {
		Source        string `json:"source"`
		Target        string `json:"target"`
		Type          string `json:"type"`
		Project       string `json:"project"`
		TargetProject string `json:"target_project"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid JSON body", 400)
		return
	}
	if body.TargetProject == "" {
		body.TargetProject = body.Project
	}

	sourceNote, err := h.store.GetNote(body.Project, body.Source)
	if err != nil {
		debugf("get source note for link delete %s: %v", body.Source, err)
		jsonError(w, "source note not found", 404)
		return
	}
	targetNote, err := h.store.GetNote(body.TargetProject, body.Target)
	if err != nil {
		debugf("get target note for link delete %s: %v", body.Target, err)
		jsonError(w, "target note not found", 404)
		return
	}

	// Modify both in memory before saving.
	originalSourceLinks := make([]model.Link, len(sourceNote.Links))
	copy(originalSourceLinks, sourceNote.Links)

	sourceNote.Links = removeLinkFiltered(sourceNote.Links, body.Target, body.Type)
	targetNote.Links = removeLinkFiltered(targetNote.Links, body.Source, body.Type)

	if err := h.store.UpdateNote(sourceNote); err != nil {
		debugf("save source note after link delete: %v", err)
		jsonError(w, "failed to update source note", 500)
		return
	}
	if err := h.store.UpdateNote(targetNote); err != nil {
		// Rollback source.
		sourceNote.Links = originalSourceLinks
		if rbErr := h.store.UpdateNote(sourceNote); rbErr != nil {
			debugf("rollback source note failed: %v", rbErr)
		}
		debugf("save target note after link delete: %v", err)
		jsonError(w, "failed to update target note (source rolled back)", 500)
		return
	}

	jsonResponse(w, map[string]bool{"ok": true})
}
