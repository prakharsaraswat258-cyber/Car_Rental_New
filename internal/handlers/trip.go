package handlers

import (
	"errors"
	"net/http"
	"os"
	"path"
	"strconv"

	"saarthi/internal/domain"
	"saarthi/internal/store"
)

// AngleView is one tile on the check-in / return checklist.
type AngleView struct {
	Code, Phase, Angle, Label, Hint string
	Photo                           *store.TripPhoto
	Required                        bool
}

func angleLabel(a string) string {
	switch a {
	case "front":
		return "Front"
	case "back":
		return "Back"
	case "left":
		return "Left side"
	case "right":
		return "Right side"
	case "odometer":
		return "Odometer"
	}
	return a
}

func angleHint(a string) string {
	switch a {
	case "front":
		return "Take the front too, so old scratches are on record."
	case "back":
		return "Take the back too, so old scratches are on record."
	case "left":
		return "Take the left side too, so old scratches are on record."
	case "right":
		return "Take the right side too, so old scratches are on record."
	}
	return "A photo of the odometer helps if there's a question about kilometres."
}

// CheckinData feeds the check-in and return pages.
type CheckinData struct {
	B          store.Booking
	Phase      string
	Title      string
	Lead       string
	Angles     []AngleView
	NeedsOdo   bool
	Odo        int
	OdoMin     int
	SwipeLabel string
	DoneLabel  string
	Action     string
}

func (h *Handlers) checkinData(b store.Booking, phase string, photos []store.TripPhoto) CheckinData {
	d := CheckinData{B: b, Phase: phase, NeedsOdo: b.VehicleType != "cycle"}
	have := map[string]*store.TripPhoto{}
	for i := range photos {
		if photos[i].Phase == phase {
			have[photos[i].Angle] = &photos[i]
		}
	}
	for _, a := range store.TripAngles {
		if a == "odometer" && !d.NeedsOdo {
			continue
		}
		d.Angles = append(d.Angles, AngleView{Code: b.Code, Phase: phase, Angle: a, Label: angleLabel(a), Hint: angleHint(a),
			Photo: have[a], Required: a != "odometer"})
	}
	if phase == "checkin" {
		d.Title, d.Lead = "Check in", "Four photos and the odometer, then swipe to start. Photos are timestamped so nobody can blame you later."
		d.SwipeLabel, d.DoneLabel, d.Action = "Swipe to start trip", "Starting…", "/trips/"+b.Code+"/start"
		d.Odo = b.OdoStart
	} else {
		d.Title, d.Lead = "Return", "Same four photos, then the odometer. The shop inspects right after."
		d.SwipeLabel, d.DoneLabel, d.Action = "Swipe to end trip", "Ending…", "/trips/"+b.Code+"/end"
		d.OdoMin = b.OdoStart
	}
	return d
}

// CheckinPage shows the angle checklist for "checkin" or "return".
func (h *Handlers) CheckinPage(phase string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, ok := h.renterBooking(w, r)
		if !ok {
			return
		}
		want := domain.StatusReady
		if phase == "return" {
			want = domain.StatusActive
		}
		if b.Status != want {
			http.Redirect(w, r, "/trips/"+b.Code, http.StatusSeeOther)
			return
		}
		photos, err := h.Store.TripPhotos(r.Context(), b.ID)
		if err != nil {
			h.serverError(w, r, err)
			return
		}
		title := "Check in"
		if phase == "return" {
			title = "Return"
		}
		h.render(w, r, http.StatusOK, "checkin", title+" · "+b.Code+" · Saarthi", "trips", h.checkinData(b, phase, photos))
	}
}

// UploadTripPhoto stores one angle photo and returns its tile.
func (h *Handlers) UploadTripPhoto(w http.ResponseWriter, r *http.Request) {
	b, ok := h.renterBooking(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, store.MaxUploadBytes+1<<20)
	phase, angle := r.FormValue("phase"), r.FormValue("angle")
	if (phase == "checkin" && b.Status != domain.StatusReady) || (phase == "return" && b.Status != domain.StatusActive) ||
		(phase != "checkin" && phase != "return") || !contains(store.TripAngles, angle) {
		badRequest(w, "Photos can't be added at this step.")
		return
	}
	file, _, err := r.FormFile("photo")
	if err != nil {
		badRequest(w, "Choose a photo to upload. Max 8 MB.")
		return
	}
	defer file.Close()
	id := store.NewID()
	rel := path.Join("trips", b.ID, phase+"-"+angle+"-"+id[:8]+".jpg")
	if err := h.Store.SaveImage(file, rel); err != nil {
		switch {
		case errors.Is(err, store.ErrTooBig):
			badRequest(w, "That photo is over 8 MB. Try again with a smaller one.")
		case errors.Is(err, store.ErrNotImage):
			badRequest(w, "That file isn't a JPEG or PNG photo.")
		default:
			h.serverError(w, r, err)
		}
		return
	}
	p, old, err := h.Store.SaveTripPhoto(r.Context(), b.ID, phase, angle, rel)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	if old != "" && old != rel {
		h.Store.RemoveUpload(old)
	}
	h.Views.Fragment(w, http.StatusOK, "angle-tile", AngleView{Code: b.Code, Phase: phase, Angle: angle, Label: angleLabel(angle),
		Hint: angleHint(angle), Photo: &p, Required: angle != "odometer"})
}

// StartTrip is the swipe at pickup.
func (h *Handlers) StartTrip(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r.Context())
	b, err := h.Store.StartTrip(r.Context(), u.ID, r.PathValue("code"), formInt(r, "odometer"))
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		h.tripError(w, r, err)
		return
	}
	setFlash(w, Toast{Message: "Trip started. Ride safe.", Tone: "good"})
	redirect(w, r, "/trips/"+b.Code)
}

// EndTrip is the swipe at return.
func (h *Handlers) EndTrip(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r.Context())
	b, err := h.Store.EndTrip(r.Context(), u.ID, r.PathValue("code"), formInt(r, "odometer"))
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		h.tripError(w, r, err)
		return
	}
	setFlash(w, Toast{Message: "Returned. The shop is inspecting now.", Tone: "good"})
	redirect(w, r, "/trips/"+b.Code)
}

func (h *Handlers) tripError(w http.ResponseWriter, r *http.Request, err error) {
	if IsHTMX(r) || r.Header.Get("X-Requested-With") == "fetch" {
		toastHeader(w, Toast{Message: friendly(err), Tone: "bad"})
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	setFlash(w, Toast{Message: friendly(err), Tone: "bad"})
	http.Redirect(w, r, safeBack(r), http.StatusSeeOther)
}

// Review stores a 1–5 star review after a closed trip.
func (h *Handlers) Review(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r.Context())
	rating, _ := strconv.Atoi(r.FormValue("rating"))
	err := h.Store.AddReview(r.Context(), u.ID, r.PathValue("code"), rating, r.FormValue("body"))
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		setFlash(w, Toast{Message: "Pick 1 to 5 stars to leave a review.", Tone: "warn"})
	} else {
		setFlash(w, Toast{Message: "Thanks. Your review helps the next renter.", Tone: "good"})
	}
	redirect(w, r, "/trips/"+r.PathValue("code"))
}

// TripPhoto serves a proof photo to the renter, the shop or an admin.
func (h *Handlers) TripPhoto(w http.ResponseWriter, r *http.Request) {
	p, err := h.Store.TripPhotoByID(r.Context(), r.PathValue("id"))
	u := UserFrom(r.Context())
	if err != nil || u == nil || !(u.ID == p.RenterID || u.ID == p.ShopOwnerID || u.IsAdmin()) {
		http.NotFound(w, r)
		return
	}
	h.serveUpload(w, r, p.Path, "private, max-age=3600")
}

// VehiclePhoto serves a public vehicle photo.
func (h *Handlers) VehiclePhoto(w http.ResponseWriter, r *http.Request) {
	p, err := h.Store.VehiclePhoto(r.Context(), r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.serveUpload(w, r, p.Path, "public, max-age=86400")
}

func (h *Handlers) serveUpload(w http.ResponseWriter, r *http.Request, rel, cache string) {
	full, err := h.Store.UploadPath(rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", cache)
	http.ServeContent(w, r, "", st.ModTime(), f)
}
