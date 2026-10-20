package store

import (
	"context"
	"strings"

	"saarthi/internal/domain/askparser"
)

// City is a place Saarthi operates in.
type City struct {
	ID, Name, Slug string
	Lat, Lng       float64
	Aliases        string
	Areas          []Area
}

// Area is a neighbourhood inside a city.
type Area struct {
	ID, CityID, Name string
	Lat, Lng         float64
	Aliases          string
}

// Cities returns every city with its areas, in name order.
func (s *Store) Cities(ctx context.Context) ([]City, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, slug, aliases, lat, lng FROM cities ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cities []City
	idx := map[string]int{}
	for rows.Next() {
		var c City
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Aliases, &c.Lat, &c.Lng); err != nil {
			return nil, err
		}
		idx[c.ID] = len(cities)
		cities = append(cities, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	arows, err := s.DB.QueryContext(ctx, `SELECT id, city_id, name, aliases, lat, lng FROM areas ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer arows.Close()
	for arows.Next() {
		var a Area
		if err := arows.Scan(&a.ID, &a.CityID, &a.Name, &a.Aliases, &a.Lat, &a.Lng); err != nil {
			return nil, err
		}
		if i, ok := idx[a.CityID]; ok {
			cities[i].Areas = append(cities[i].Areas, a)
		}
	}
	return cities, arows.Err()
}

// CityBySlug finds one city.
func (s *Store) CityBySlug(ctx context.Context, slug string) (City, error) {
	cities, err := s.Cities(ctx)
	if err != nil {
		return City{}, err
	}
	for _, c := range cities {
		if c.Slug == slug {
			return c, nil
		}
	}
	return City{}, ErrNotFound
}

// AreaByID finds one area and its city.
func (s *Store) AreaByID(ctx context.Context, id string) (Area, City, error) {
	cities, err := s.Cities(ctx)
	if err != nil {
		return Area{}, City{}, err
	}
	for _, c := range cities {
		for _, a := range c.Areas {
			if a.ID == id {
				return a, c, nil
			}
		}
	}
	return Area{}, City{}, ErrNotFound
}

// Gazetteer lists every city and area with aliases for the Ask parser.
func (s *Store) Gazetteer(ctx context.Context) (askparser.Gazetteer, error) {
	cities, err := s.Cities(ctx)
	if err != nil {
		return nil, err
	}
	var g askparser.Gazetteer
	for _, c := range cities {
		g = append(g, askparser.Place{CitySlug: c.Slug, CityName: c.Name, Aliases: aliasList(c.Aliases, c.Name)})
		for _, a := range c.Areas {
			g = append(g, askparser.Place{CitySlug: c.Slug, CityName: c.Name, AreaID: a.ID, AreaName: a.Name,
				Aliases: aliasList(a.Aliases, a.Name)})
		}
	}
	return g, nil
}

func aliasList(csv, name string) []string {
	out := []string{strings.ToLower(name)}
	for _, a := range strings.Split(csv, ",") {
		if a = strings.TrimSpace(strings.ToLower(a)); a != "" && a != out[0] {
			out = append(out, a)
		}
	}
	return out
}
