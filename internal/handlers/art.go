package handlers

import "html/template"

// Original single-line vehicle silhouettes (no third-party artwork). They
// draw in currentColor with one saffron accent, so they follow the theme.
// Rendered with {{art "scooter"}}; names are fixed, never user input.

const artOpen = `<svg viewBox="0 0 240 150" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">`
const artGround = `<path d="M18 140 H222" stroke-dasharray="2 9" opacity=".45"/>`

var artSVG = map[string]string{
	"scooter": artOpen + artGround +
		`<path d="M28 102 C28 82 46 72 72 70 L120 68 C130 68 136 74 138 84 L142 104 H100"/>` +
		`<path d="M78 62 C88 54 112 54 122 62"/>` +
		`<path d="M142 104 L152 62 C154 52 160 46 168 44"/>` +
		`<path d="M168 44 L162 30 M152 30 H176"/>` +
		`<path d="M160 60 L184 116"/>` +
		`<circle cx="62" cy="116" r="20"/><circle cx="62" cy="116" r="4"/>` +
		`<circle cx="184" cy="116" r="20"/><circle cx="184" cy="116" r="4"/>` +
		`<circle class="accent" cx="173" cy="40" r="4"/>` +
		`</svg>`,

	"ev-scooter": artOpen + artGround +
		`<path d="M28 102 C28 82 46 72 72 70 L120 68 C130 68 136 74 138 84 L142 104 H100"/>` +
		`<path d="M78 62 C88 54 112 54 122 62"/>` +
		`<path d="M142 104 L152 62 C154 52 160 46 168 44"/>` +
		`<path d="M168 44 L162 30 M152 30 H176"/>` +
		`<path d="M160 60 L184 116"/>` +
		`<circle cx="62" cy="116" r="20"/><circle cx="62" cy="116" r="4"/>` +
		`<circle cx="184" cy="116" r="20"/><circle cx="184" cy="116" r="4"/>` +
		`<path class="accent" d="M98 76 L88 90 H98 L92 102 L108 86 H98 L104 76 Z"/>` +
		`</svg>`,

	"bike": artOpen + artGround +
		`<circle cx="58" cy="112" r="24"/><circle cx="58" cy="112" r="5"/>` +
		`<circle cx="186" cy="112" r="24"/><circle cx="186" cy="112" r="5"/>` +
		`<path d="M58 112 L92 84 L134 86 L152 110"/>` +
		`<path d="M98 70 C104 58 132 54 146 62 L144 76 L100 78 Z"/>` +
		`<path d="M60 74 C70 68 88 68 98 72"/>` +
		`<path d="M186 112 L162 54"/>` +
		`<path d="M152 50 L174 46"/>` +
		`<path d="M104 90 H130 V106 H104 Z"/>` +
		`<path d="M80 104 L140 108"/>` +
		`<path d="M32 96 C38 84 56 80 70 84"/>` +
		`<path d="M174 86 C186 82 198 86 206 94"/>` +
		`<circle class="accent" cx="168" cy="68" r="7"/>` +
		`</svg>`,

	"car": artOpen + artGround +
		`<path d="M24 108 V92 C24 84 30 80 38 78 L64 74 L92 52 C96 48 102 46 108 46 H158 C166 46 172 50 176 56 L190 74 L206 78 C214 80 218 86 218 94 V108"/>` +
		`<path d="M24 108 H40 M84 108 H158 M202 108 H218"/>` +
		`<path d="M40 108 A22 22 0 0 1 84 108 M158 108 A22 22 0 0 1 202 108"/>` +
		`<circle cx="62" cy="112" r="17"/><circle cx="62" cy="112" r="5"/>` +
		`<circle cx="180" cy="112" r="17"/><circle cx="180" cy="112" r="5"/>` +
		`<path d="M78 74 L99 54 H132 V74 Z"/>` +
		`<path d="M140 54 H159 C163 54 166 56 168 59 L178 74 H140 Z"/>` +
		`<path d="M136 80 V100 M118 86 H128"/>` +
		`<path d="M26 86 H32"/>` +
		`<path class="accent" d="M206 86 H214"/>` +
		`</svg>`,

	"ev-car": artOpen + artGround +
		`<path d="M22 108 V86 C22 78 28 72 36 70 L62 66 L84 44 C88 40 94 38 100 38 H160 C168 38 174 42 178 48 L192 66 L208 70 C216 72 220 78 220 86 V108"/>` +
		`<path d="M22 108 H40 M84 108 H158 M202 108 H220"/>` +
		`<path d="M40 108 A22 22 0 0 1 84 108 M158 108 A22 22 0 0 1 202 108"/>` +
		`<circle cx="62" cy="112" r="17"/><circle cx="62" cy="112" r="5"/>` +
		`<circle cx="180" cy="112" r="17"/><circle cx="180" cy="112" r="5"/>` +
		`<path d="M76 66 L92 46 H128 V66 Z"/>` +
		`<path d="M136 46 H160 C164 46 167 48 169 51 L180 66 H136 Z"/>` +
		`<path d="M84 30 H170" opacity=".7"/>` +
		`<path class="accent" d="M120 74 L110 88 H120 L114 100 L130 84 H120 L126 74 Z"/>` +
		`<path d="M22 92 C12 92 10 100 10 108"/>` +
		`</svg>`,

	"cycle": artOpen + artGround +
		`<circle cx="60" cy="108" r="30"/><circle cx="60" cy="108" r="3"/>` +
		`<circle cx="182" cy="108" r="30"/><circle cx="182" cy="108" r="3"/>` +
		`<path d="M60 108 L98 66 H160 L182 108"/>` +
		`<path d="M98 66 L118 108 L160 66"/>` +
		`<path d="M118 108 H60"/>` +
		`<path d="M98 66 L92 50 M82 48 H102"/>` +
		`<path d="M160 66 L166 48 M158 46 C166 42 176 42 182 48"/>` +
		`<circle class="accent" cx="118" cy="108" r="7"/>` +
		`</svg>`,
}

// art returns the inline SVG for a vehicle art name.
func art(name string) template.HTML {
	s, ok := artSVG[name]
	if !ok {
		s = artSVG["scooter"]
	}
	return template.HTML(`<span class="art">` + s + `</span>`) // constant markup, never user input
}

// wheelSVG is the brand wheel used by loaders.
const wheelSVG = `<svg class="wheel" viewBox="0 0 32 32" fill="none" aria-hidden="true" focusable="false">` +
	`<circle cx="16" cy="16" r="11.5" stroke="currentColor" stroke-width="2.5"/>` +
	`<circle cx="16" cy="16" r="2.8" fill="currentColor"/>` +
	`<path d="M16 13.4V5M18.47 15.2l7.99-2.6M17.53 18.1l4.94 6.8M14.47 18.1l-4.94 6.8M13.53 15.2l-7.99-2.6" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>` +
	`</svg>`

func wheel() template.HTML { return template.HTML(wheelSVG) }
