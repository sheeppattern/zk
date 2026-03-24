package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/sheeppattern/nete/internal/model"
	"github.com/sheeppattern/nete/internal/store"
)

type serveHandler struct {
	store *store.Store
}

// apiMemoView is the JSON API representation of a Memo.
type apiMemoView struct {
	ID       int64          `json:"id"`
	Title    string         `json:"title"`
	Content  string         `json:"content"`
	Layer    string         `json:"layer"`
	Tags     []string       `json:"tags"`
	NoteID   int64          `json:"note_id"`
	Metadata model.Metadata `json:"metadata"`
	Links    *apiMemoLinks  `json:"links,omitempty"`
}

type apiMemoLinks struct {
	Outgoing []model.Link `json:"outgoing"`
	Incoming []model.Link `json:"incoming"`
}

func (h *serveHandler) attachLinks(view *apiMemoView) {
	outgoing, incoming, err := h.store.ListLinks(view.ID)
	if err != nil {
		return
	}
	if outgoing == nil {
		outgoing = []model.Link{}
	}
	if incoming == nil {
		incoming = []model.Link{}
	}
	view.Links = &apiMemoLinks{Outgoing: outgoing, Incoming: incoming}
}

func toAPIMemoView(m *model.Memo) apiMemoView {
	layer := m.Layer
	if layer == "" {
		layer = "concrete"
	}
	tags := m.Tags
	if tags == nil {
		tags = []string{}
	}
	return apiMemoView{
		ID:       m.ID,
		Title:    m.Title,
		Content:  m.Content,
		Layer:    layer,
		Tags:     tags,
		NoteID:   m.NoteID,
		Metadata: m.Metadata,
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

// GET /api/notes
func (h *serveHandler) handleNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonError(w, "method not allowed", 405)
		return
	}
	notes, err := h.store.ListNotes()
	if err != nil {
		debugf("list notes: %v", err)
		jsonError(w, "failed to list notes", 500)
		return
	}
	if notes == nil {
		notes = []*model.Note{}
	}
	jsonResponse(w, notes)
}

// GET /api/memos?note=X
func (h *serveHandler) handleMemos(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonError(w, "method not allowed", 405)
		return
	}
	noteIDStr := r.URL.Query().Get("note")
	var noteID int64
	if noteIDStr != "" {
		var err error
		noteID, err = strconv.ParseInt(noteIDStr, 10, 64)
		if err != nil {
			jsonError(w, "invalid note parameter", 400)
			return
		}
	}

	var memos []*model.Memo
	var err error
	if noteID != 0 {
		memos, err = h.store.ListMemos(noteID)
	} else {
		memos, err = h.store.ListAllMemos()
	}
	if err != nil {
		debugf("list memos: %v", err)
		jsonError(w, "failed to list memos", 500)
		return
	}
	views := make([]apiMemoView, 0, len(memos))
	for _, m := range memos {
		views = append(views, toAPIMemoView(m))
	}
	jsonResponse(w, views)
}

// GET/PUT/DELETE /api/memo?id=X
func (h *serveHandler) handleMemo(w http.ResponseWriter, r *http.Request) {
	memoIDStr := r.URL.Query().Get("id")
	memoID, err := strconv.ParseInt(memoIDStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid memo id", 400)
		return
	}

	switch r.Method {
	case "GET":
		memo, err := h.store.GetMemo(memoID)
		if err != nil {
			debugf("get memo %d: %v", memoID, err)
			jsonError(w, "memo not found", 404)
			return
		}
		view := toAPIMemoView(memo)
		h.attachLinks(&view)
		jsonResponse(w, view)

	case "PUT":
		var body struct {
			Title   *string  `json:"title"`
			Content *string  `json:"content"`
			Tags    []string `json:"tags"`
			Summary *string  `json:"summary"`
			Status  *string  `json:"status"`
			Source  *string  `json:"source"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid JSON body", 400)
			return
		}
		memo, err := h.store.GetMemo(memoID)
		if err != nil {
			debugf("get memo for update %d: %v", memoID, err)
			jsonError(w, "memo not found", 404)
			return
		}
		if body.Title != nil {
			memo.Title = *body.Title
		}
		if body.Content != nil {
			memo.Content = *body.Content
		}
		if body.Tags != nil {
			memo.Tags = body.Tags
		}
		if body.Summary != nil {
			memo.Metadata.Summary = *body.Summary
		}
		if body.Status != nil {
			memo.Metadata.Status = *body.Status
		}
		if body.Source != nil {
			memo.Metadata.Source = *body.Source
		}
		if err := h.store.UpdateMemo(memo); err != nil {
			debugf("update memo %d: %v", memoID, err)
			jsonError(w, "failed to save memo", 500)
			return
		}
		view := toAPIMemoView(memo)
		h.attachLinks(&view)
		jsonResponse(w, view)

	case "DELETE":
		if err := h.store.DeleteMemo(memoID); err != nil {
			debugf("delete memo %d: %v", memoID, err)
			jsonError(w, "failed to delete memo", 500)
			return
		}
		jsonResponse(w, map[string]bool{"ok": true})

	default:
		jsonError(w, "method not allowed", 405)
	}
}

// GET /api/search?q=X&note=X
func (h *serveHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonError(w, "method not allowed", 405)
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		jsonError(w, "missing query parameter 'q'", 400)
		return
	}

	noteIDStr := r.URL.Query().Get("note")
	var noteID int64
	if noteIDStr != "" {
		noteID, _ = strconv.ParseInt(noteIDStr, 10, 64)
	}

	opts := store.SearchOptions{
		NoteID: noteID,
	}
	memos, err := h.store.SearchMemos(query, opts)
	if err != nil {
		debugf("search memos: %v", err)
		jsonError(w, "search failed", 500)
		return
	}

	views := make([]apiMemoView, 0, len(memos))
	for _, m := range memos {
		views = append(views, toAPIMemoView(m))
	}
	jsonResponse(w, views)
}

// GET /api/all-data
func (h *serveHandler) handleAllData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonError(w, "method not allowed", 405)
		return
	}

	notes, _ := h.store.ListNotes()
	if notes == nil {
		notes = []*model.Note{}
	}

	allMemos, _ := h.store.ListAllMemos()
	views := make([]apiMemoView, 0, len(allMemos))
	for _, m := range allMemos {
		views = append(views, toAPIMemoView(m))
	}

	jsonResponse(w, map[string]interface{}{
		"notes": notes,
		"memos": views,
	})
}

// POST /api/tag  body: {memo_id, tag, action: "add"|"remove"}
func (h *serveHandler) handleTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "method not allowed", 405)
		return
	}
	var body struct {
		MemoID int64  `json:"memo_id"`
		Tag    string `json:"tag"`
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid JSON body", 400)
		return
	}

	memo, err := h.store.GetMemo(body.MemoID)
	if err != nil {
		debugf("get memo for tag %d: %v", body.MemoID, err)
		jsonError(w, "memo not found", 404)
		return
	}

	switch body.Action {
	case "add":
		for _, t := range memo.Tags {
			if t == body.Tag {
				jsonResponse(w, toAPIMemoView(memo))
				return
			}
		}
		memo.Tags = append(memo.Tags, body.Tag)
	case "remove":
		filtered := make([]string, 0, len(memo.Tags))
		for _, t := range memo.Tags {
			if t != body.Tag {
				filtered = append(filtered, t)
			}
		}
		memo.Tags = filtered
	default:
		jsonError(w, fmt.Sprintf("invalid action %q, expected 'add' or 'remove'", body.Action), 400)
		return
	}

	if err := h.store.UpdateMemo(memo); err != nil {
		debugf("update memo tags %d: %v", body.MemoID, err)
		jsonError(w, "failed to update tags", 500)
		return
	}
	jsonResponse(w, toAPIMemoView(memo))
}

// DELETE /api/link  body: {source, target, type}
func (h *serveHandler) handleLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		jsonError(w, "method not allowed", 405)
		return
	}
	var body struct {
		Source int64  `json:"source"`
		Target int64  `json:"target"`
		Type   string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid JSON body", 400)
		return
	}

	relType := body.Type
	if relType == "" {
		relType = "related"
	}

	if err := h.store.RemoveLink(body.Source, body.Target, relType); err != nil {
		debugf("remove link %d→%d: %v", body.Source, body.Target, err)
		jsonError(w, "failed to remove link", 500)
		return
	}

	jsonResponse(w, map[string]bool{"ok": true})
}

