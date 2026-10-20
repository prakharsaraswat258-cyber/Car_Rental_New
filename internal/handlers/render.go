package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"saarthi/internal/domain"
)

// Views renders html/template pages. Each page under templates/pages is
// parsed together with every layout and partial. Pages in pages/provider
// and pages/admin use the "console" layout, everything else "app" (D-024).
type Views struct {
	fsys fs.FS // rooted at web/: templates/… and static/…
	dev  bool
	log  *slog.Logger

	mu     sync.RWMutex
	pages  map[string]*template.Template
	shared *template.Template // layouts + partials, for fragment renders

	assetMu  sync.Mutex
	assetVer map[string]string
}

// NewViews parses all templates once. In dev mode they are re-parsed on
// every render so edits show on refresh (D-019).
func NewViews(fsys fs.FS, dev bool, log *slog.Logger) (*Views, error) {
	v := &Views{fsys: fsys, dev: dev, log: log, assetVer: map[string]string{}}
	if err := v.load(); err != nil {
		return nil, err
	}
	return v, nil
}

func (v *Views) load() error {
	base, err := template.New("").Funcs(v.funcs()).ParseFS(v.fsys,
		"templates/layouts/*.html", "templates/partials/*.html")
	if err != nil {
		return fmt.Errorf("parse layouts/partials: %w", err)
	}
	pages := map[string]*template.Template{}
	err = fs.WalkDir(v.fsys, "templates/pages", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		name := strings.TrimSuffix(strings.TrimPrefix(path, "templates/pages/"), ".html")
		t, err := base.Clone()
		if err != nil {
			return err
		}
		if _, err := t.ParseFS(v.fsys, path); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		pages[name] = t
		return nil
	})
	if err != nil {
		return err
	}
	shared, err := base.Clone()
	if err != nil {
		return err
	}
	v.mu.Lock()
	v.pages, v.shared = pages, shared
	v.mu.Unlock()
	return nil
}

func layoutFor(page string) string {
	if strings.HasPrefix(page, "provider/") || strings.HasPrefix(page, "admin/") {
		return "console"
	}
	return "app"
}

// IsHTMX reports an htmx request that wants a fragment (not a boosted
// navigation or a history restore, which need the full page).
func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true" &&
		r.Header.Get("HX-Boosted") != "true" &&
		r.Header.Get("HX-History-Restore-Request") != "true"
}

// Page renders a full page, or only its "content" block for htmx.
func (v *Views) Page(w http.ResponseWriter, r *http.Request, status int, page string, data any) {
	if v.dev {
		if err := v.load(); err != nil {
			v.fail(w, err)
			return
		}
	}
	v.mu.RLock()
	t, ok := v.pages[page]
	v.mu.RUnlock()
	if !ok {
		v.fail(w, fmt.Errorf("no page template %q", page))
		return
	}
	name := layoutFor(page)
	if IsHTMX(r) {
		name = "content"
	}
	v.execute(w, status, t, name, data)
}

// Fragment renders one named partial template (e.g. "price-box").
func (v *Views) Fragment(w http.ResponseWriter, status int, name string, data any) {
	if v.dev {
		if err := v.load(); err != nil {
			v.fail(w, err)
			return
		}
	}
	v.mu.RLock()
	t := v.shared
	v.mu.RUnlock()
	v.execute(w, status, t, name, data)
}

func (v *Views) execute(w http.ResponseWriter, status int, t *template.Template, name string, data any) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		v.fail(w, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Add("Vary", "HX-Request")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

func (v *Views) fail(w http.ResponseWriter, err error) {
	v.log.Error("render", "err", err)
	msg := "Something went wrong on our side. Please try again."
	if v.dev {
		msg = err.Error()
	}
	http.Error(w, msg, http.StatusInternalServerError)
}

// --- template funcs ----------------------------------------------------------

var iconName = regexp.MustCompile(`^[a-z0-9-]+$`)

func (v *Views) funcs() template.FuncMap {
	return template.FuncMap{
		"rupees": rupees,
		"grouped": func(n any) string {
			return domain.Grouped(toInt64(n))
		},
		"asset": v.asset,
		// icon renders a Lucide symbol from the sprite. Names are fixed in
		// templates (never user input) and still checked.
		"icon": func(name string, class ...string) template.HTML {
			if !iconName.MatchString(name) {
				return ""
			}
			cls := "icon"
			if len(class) > 0 {
				cls += " " + template.HTMLEscapeString(strings.Join(class, " "))
			}
			return template.HTML(`<svg class="` + cls + `" aria-hidden="true" focusable="false"><use href="` +
				v.asset("/static/img/icons.svg") + `#` + name + `"></use></svg>`)
		},
		"date":      domain.DayText,
		"datefull":  func(t time.Time) string { return t.In(domain.IST).Format("Mon 2 Jan 2006") },
		"clock":     domain.Clock,
		"when":      func(t time.Time) string { return domain.DayText(t) + ", " + domain.Clock(t) },
		"isodate":   func(t time.Time) string { return t.In(domain.IST).Format("2006-01-02") },
		"local":     domain.FormatLocal,
		"rangeText": domain.RangeText,
		"durWords":  domain.DurationWords,
		"typeLabel": domain.TypeLabel,
		"art":       art,
		"wheel":     wheel,
		"dict":      dict,
		"add":       func(a, b int) int { return a + b },
		"sub":       func(a, b int) int { return a - b },
		"mul":       func(a, b int) int { return a * b },
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		"lower": strings.ToLower,
		"initial": func(s string) string {
			r := []rune(strings.TrimSpace(s))
			if len(r) == 0 {
				return "?"
			}
			return strings.ToUpper(string(r[0]))
		},
		"pct":        func(f float64) string { return fmt.Sprintf("%.3f%%", f) },
		"stepLabels": func() [6]string { return domain.TripSteps },
		"has":        contains,
		// tel builds a tel: link; only digits and + survive, so it is safe.
		"tel": func(s string) template.URL {
			return template.URL("tel:" + strings.Map(func(r rune) rune {
				if r == '+' || r >= '0' && r <= '9' {
					return r
				}
				return -1
			}, s))
		},
		"json":       toJSON,
		"angleLabel": angleLabel,
		"split": func(s, sep string) []string {
			if strings.TrimSpace(s) == "" {
				return nil
			}
			parts := strings.Split(s, sep)
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			return parts
		},
		"year": func() int { return time.Now().In(domain.IST).Year() },
	}
}

func rupees(v any) string { return domain.Paise(toInt64(v)).String() }

func toInt64(v any) int64 {
	switch n := v.(type) {
	case domain.Paise:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	}
	return 0
}

// dict builds a map for passing several values to a partial:
// {{template "chip" dict "Label" "Scooter" "Tone" "road"}}
func dict(kv ...any) (map[string]any, error) {
	if len(kv)%2 != 0 {
		return nil, fmt.Errorf("dict needs key/value pairs")
	}
	m := make(map[string]any, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict key %v is not a string", kv[i])
		}
		m[k] = kv[i+1]
	}
	return m, nil
}

// asset appends ?v=<content hash> to a /static/ path so browsers can cache
// it forever (D-027). In dev mode it returns the path unchanged.
func (v *Views) asset(path string) string {
	if v.dev || !strings.HasPrefix(path, "/static/") {
		return path
	}
	v.assetMu.Lock()
	defer v.assetMu.Unlock()
	if ver, ok := v.assetVer[path]; ok {
		return ver
	}
	b, err := fs.ReadFile(v.fsys, strings.TrimPrefix(path, "/"))
	if err != nil {
		return path
	}
	sum := sha256.Sum256(b)
	ver := path + "?v=" + hex.EncodeToString(sum[:])[:10]
	v.assetVer[path] = ver
	return ver
}
