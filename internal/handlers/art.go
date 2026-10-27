package handlers

import "html/template"

// One original single-line illustration set (no third-party artwork):
// 1.5 px non-scaling strokes in currentColor (--ink-2) with one saffron
// accent, so every drawing follows the theme. Rendered with {{art "name"}};
// names are fixed in templates, never user input.

const artOpen = `<svg viewBox="0 0 240 150" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">`
const artGround = `<path d="M18 140 H222" stroke-dasharray="2 7" opacity=".5"/>`

const (
	scooterSide = `<path d="M28 102 C28 82 46 72 72 70 L120 68 C130 68 136 74 138 84 L142 104 H100"/>` +
		`<path d="M78 62 C88 54 112 54 122 62"/>` +
		`<path d="M142 104 L152 62 C154 52 160 46 168 44"/>` +
		`<path d="M168 44 L162 30 M152 30 H176"/>` +
		`<path d="M160 60 L184 116"/>` +
		`<circle cx="62" cy="116" r="20"/><circle cx="62" cy="116" r="4"/>` +
		`<circle cx="184" cy="116" r="20"/><circle cx="184" cy="116" r="4"/>`
	scooterFront = `<path d="M84 38 H156 M76 32 l8 6 M164 32 l-8 6"/>` +
		`<path d="M106 38 L102 58 H138 L134 38"/>` +
		`<path d="M100 62 C94 82 96 102 106 114 H134 C144 102 146 82 140 62 Z"/>` +
		`<path d="M112 114 V138 H128 V114"/>`
	bikeSide = `<circle cx="58" cy="112" r="24"/><circle cx="58" cy="112" r="5"/>` +
		`<circle cx="186" cy="112" r="24"/><circle cx="186" cy="112" r="5"/>` +
		`<path d="M58 112 L92 84 L134 86 L152 110"/>` +
		`<path d="M98 70 C104 58 132 54 146 62 L144 76 L100 78 Z"/>` +
		`<path d="M60 74 C70 68 88 68 98 72"/>` +
		`<path d="M186 112 L162 54 M152 50 L174 46"/>` +
		`<path d="M104 90 H130 V106 H104 Z M80 104 L140 108"/>` +
		`<path d="M32 96 C38 84 56 80 70 84 M174 86 C186 82 198 86 206 94"/>`
	bikeFront = `<path d="M74 44 H166 M66 38 l8 6 M174 38 l-8 6"/>` +
		`<path d="M96 44 L104 72 M144 44 L136 72"/>` +
		`<path d="M104 78 C98 94 104 108 112 112 H128 C136 108 142 94 136 78 Z"/>` +
		`<path d="M112 112 V138 H128 V112"/>`
	carSide = `<path d="M24 108 V92 C24 84 30 80 38 78 L64 74 L92 52 C96 48 102 46 108 46 H158 C166 46 172 50 176 56 L190 74 L206 78 C214 80 218 86 218 94 V108"/>` +
		`<path d="M24 108 H40 M84 108 H158 M202 108 H218"/>` +
		`<path d="M40 108 A22 22 0 0 1 84 108 M158 108 A22 22 0 0 1 202 108"/>` +
		`<circle cx="62" cy="112" r="17"/><circle cx="62" cy="112" r="5"/>` +
		`<circle cx="180" cy="112" r="17"/><circle cx="180" cy="112" r="5"/>` +
		`<path d="M78 74 L99 54 H132 V74 Z M140 54 H159 C163 54 166 56 168 59 L178 74 H140 Z"/>` +
		`<path d="M136 80 V100 M118 86 H128 M26 86 H32"/>`
	carFront = `<path d="M50 112 V88 C50 80 54 74 60 72 L74 48 C77 42 82 40 88 40 H152 C158 40 163 42 166 48 L180 72 C186 74 190 80 190 88 V112 Z"/>` +
		`<path d="M80 70 L90 50 H150 L160 70 Z"/>` +
		`<path d="M100 98 H140 M50 102 H190"/>` +
		`<path d="M54 112 V124 H74 V112 M166 112 V124 H186 V112"/>` +
		`<path d="M40 66 H52 M188 66 H200"/>`
	evCarSide = `<path d="M22 108 V86 C22 78 28 72 36 70 L62 66 L84 44 C88 40 94 38 100 38 H160 C168 38 174 42 178 48 L192 66 L208 70 C216 72 220 78 220 86 V108"/>` +
		`<path d="M22 108 H40 M84 108 H158 M202 108 H220"/>` +
		`<path d="M40 108 A22 22 0 0 1 84 108 M158 108 A22 22 0 0 1 202 108"/>` +
		`<circle cx="62" cy="112" r="17"/><circle cx="62" cy="112" r="5"/>` +
		`<circle cx="180" cy="112" r="17"/><circle cx="180" cy="112" r="5"/>` +
		`<path d="M76 66 L92 46 H128 V66 Z M136 46 H160 C164 46 167 48 169 51 L180 66 H136 Z"/>` +
		`<path d="M84 30 H170" opacity=".7"/><path d="M22 92 C12 92 10 100 10 108"/>`
	cycleSide = `<circle cx="60" cy="108" r="30"/><circle cx="60" cy="108" r="3"/>` +
		`<circle cx="182" cy="108" r="30"/><circle cx="182" cy="108" r="3"/>` +
		`<path d="M60 108 L98 66 H160 L182 108 M98 66 L118 108 L160 66 M118 108 H60"/>` +
		`<path d="M98 66 L92 50 M82 48 H102 M160 66 L166 48 M158 46 C166 42 176 42 182 48"/>`
	cycleFront = `<path d="M86 46 C100 40 140 40 154 46"/><path d="M120 42 V62"/>` +
		`<path d="M114 62 L116 100 M126 62 L124 100"/><path d="M114 100 V140 H126 V100"/>`
	bolt = `<path class="accent" d="M120 74 L110 88 H120 L114 100 L130 84 H120 L126 74 Z"/>`
)

var artSVG = map[string]string{
	// side views
	"scooter":    artOpen + artGround + scooterSide + `<circle class="accent" cx="173" cy="40" r="4"/></svg>`,
	"ev-scooter": artOpen + artGround + scooterSide + `<path class="accent" d="M98 76 L88 90 H98 L92 102 L108 86 H98 L104 76 Z"/></svg>`,
	"bike":       artOpen + artGround + bikeSide + `<circle class="accent" cx="168" cy="68" r="7"/></svg>`,
	"car":        artOpen + artGround + carSide + `<path class="accent" d="M206 86 H214"/></svg>`,
	"ev-car":     artOpen + artGround + evCarSide + bolt + `</svg>`,
	"cycle":      artOpen + artGround + cycleSide + `<circle class="accent" cx="118" cy="108" r="7"/></svg>`,

	// front views (gallery, camera guides)
	"scooter-front":    artOpen + artGround + scooterFront + `<circle class="accent" cx="120" cy="49" r="6"/></svg>`,
	"ev-scooter-front": artOpen + artGround + scooterFront + `<circle class="accent" cx="120" cy="49" r="6"/><path class="accent" d="M122 76 L114 88 H122 L118 98 L130 84 H122 L126 76 Z"/></svg>`,
	"bike-front":       artOpen + artGround + bikeFront + `<circle class="accent" cx="120" cy="62" r="12"/></svg>`,
	"car-front":        artOpen + artGround + carFront + `<path class="accent" d="M58 84 H80 M160 84 H182"/></svg>`,
	"ev-car-front":     artOpen + artGround + carFront + `<path class="accent" d="M58 84 H80 M160 84 H182"/><path class="accent" d="M122 82 L114 94 H122 L118 104 L130 90 H122 L126 82 Z"/></svg>`,
	"cycle-front":      artOpen + artGround + cycleFront + `<circle class="accent" cx="120" cy="68" r="4"/></svg>`,

	// gauges (camera guides for odometer / fuel)
	"gauge": artOpen + `<circle cx="120" cy="76" r="52"/><path d="M84 108 A48 48 0 1 1 156 108"/>` +
		`<path d="M120 76 L150 52" class="accent"/><circle cx="120" cy="76" r="5"/><path d="M100 104 H140 V116 H100 Z"/></svg>`,

	// illustrations
	"il-road-end": artOpen + `<path d="M30 142 L102 56 M210 142 L138 56"/><path d="M120 142 V64" stroke-dasharray="8 10"/>` +
		`<path d="M86 56 H154 M92 56 V36 M148 56 V36 M84 36 H156 V46 H84 Z"/>` +
		`<path class="accent" d="M96 36 L104 46 M112 36 L120 46 M128 36 L136 46 M144 36 L152 46"/>` +
		`<path d="M176 92 C186 82 198 82 206 92" opacity=".6"/><path d="M28 70 C40 60 52 60 64 70" opacity=".6"/></svg>`,
	"il-broken": artOpen + `<circle cx="96" cy="78" r="44"/><circle cx="96" cy="78" r="6"/>` +
		`<path d="M96 72 V38 M101 80 L134 92 M92 83 L72 112 M90 76 L58 66"/><path class="accent" d="M100 76 L118 50 M114 46 L124 54"/>` +
		`<path d="M168 112 L196 84 M190 72 C198 68 208 76 204 86 L198 90 L194 86 L188 92 L192 96 L188 100 C178 104 170 96 174 88"/>` +
		`<path d="M30 140 H210" stroke-dasharray="2 7" opacity=".5"/></svg>`,
	"il-offline": artOpen + `<path d="M74 104 H164 C182 104 192 92 192 78 C192 64 180 54 166 56 C162 38 146 28 128 30 C110 32 98 44 96 58 C82 56 66 64 66 80 C66 94 74 104 74 104 Z"/>` +
		`<path class="accent" d="M70 124 L176 30"/><path d="M98 126 H142 M108 136 H132" opacity=".6"/></svg>`,
	"il-calendar": artOpen + `<path d="M64 36 H176 V128 H64 Z M64 58 H176 M90 26 V44 M150 26 V44"/>` +
		`<path d="M78 76 H94 M102 76 H118 M126 76 H142 M150 76 H166 M78 96 H94 M126 96 H142 M150 96 H166 M78 116 H94 M102 116 H118"/>` +
		`<path class="accent" d="M100 92 L120 100 M100 100 L120 92"/></svg>`,
	"il-compare": artOpen + `<path d="M40 40 H106 V120 H40 Z M134 40 H200 V120 H134 Z"/>` +
		`<path d="M50 96 H96 M50 106 H82 M144 96 H190 M144 106 H176" opacity=".7"/>` +
		`<path d="M54 82 C54 66 92 66 92 82 M148 82 C148 66 186 66 186 82"/><path class="accent" d="M112 72 H128 M122 66 L128 72 L122 78 M128 90 H112 M118 84 L112 90 L118 96"/></svg>`,
	"il-quiet": artOpen + `<path class="accent" d="M88 92 A32 32 0 0 1 152 92"/><path d="M120 46 V54 M84 58 L90 64 M156 58 L150 64 M70 82 H78 M162 82 H170"/>` +
		`<path d="M30 92 H210"/><path d="M110 96 L80 142 M130 96 L160 142"/><path d="M120 102 V142" stroke-dasharray="6 8"/></svg>`,
	"il-checklist": artOpen + `<path d="M76 30 H164 V140 H76 Z M100 22 H140 V38 H100 Z"/>` +
		`<path class="accent" d="M90 64 L96 70 L106 58"/><path d="M116 64 H148"/>` +
		`<path class="accent" d="M90 90 L96 96 L106 84"/><path d="M116 90 H148"/>` +
		`<path d="M90 110 H104 V122 H90 Z M116 116 H148"/></svg>`,
}

// art returns the inline SVG for an art name (vehicle or illustration).
func art(name string) template.HTML {
	s, ok := artSVG[name]
	if !ok {
		s = artSVG["scooter"]
	}
	return template.HTML(`<span class="art">` + s + `</span>`) // constant markup, never user input
}

// frontArt is the front-view drawing for a side-view art name.
func frontArt(name string) string {
	if _, ok := artSVG[name+"-front"]; ok {
		return name + "-front"
	}
	return "scooter-front"
}

// wheelSVG is the brand wheel used by loaders.
const wheelSVG = `<svg class="wheel" viewBox="0 0 32 32" fill="none" aria-hidden="true" focusable="false">` +
	`<circle cx="16" cy="16" r="11.5" stroke="currentColor" stroke-width="2.5"/>` +
	`<circle cx="16" cy="16" r="2.8" fill="currentColor"/>` +
	`<path d="M16 13.4V5M18.47 15.2l7.99-2.6M17.53 18.1l4.94 6.8M14.47 18.1l-4.94 6.8M13.53 15.2l-7.99-2.6" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>` +
	`</svg>`

func wheel() template.HTML { return template.HTML(wheelSVG) }
